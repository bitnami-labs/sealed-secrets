package vault

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/physical"
	"github.com/hashicorp/vault/vault/seal/transit"
)

var (
	// ErrMissingVaultKubernetesPath is our error, if the mount path of the Kubernetes Auth Method is not provided.
	ErrMissingVaultKubernetesPath = errors.New("missing ttl for vault token")
	// ErrMissingVaultKubernetesRole is our error, if the role for the Kubernetes Auth Method is not provided.
	ErrMissingVaultKubernetesRole = errors.New("missing ttl for vault token")
	// ErrMissingVaultAuthInfo is our error, if sth. went wrong during the authentication agains Vault.
	ErrMissingVaultAuthInfo = errors.New("missing authentication information")

	// log is our customized logger.
	logger = logging.NewVaultLogger(log.Trace)

	// client is the API client for the interaction with the Vault API.
	client *api.Client

	// tokenLeaseDuration is the lease duration of the token for the interaction with vault.
	tokenLeaseDuration = 1800

	// vault vals
	vaultAddress        = getEnv("VAULT_ADDR", "http://localhost:8200")
	vaultKubernetesPath = getEnv("VAULT_KUBERNETES_PATH", "kubernetes")
	vaultKubernetesRole = getEnv("VAULT_KUBERNETES_ROLE", "default")
	vaultTransitKey     = getEnv("VAULT_TRANSIT_KEY", "sealed-secrets")
	vaultTransitPath    = getEnv("VAULT_TRANSIT_PATH", "transit")
	serviceAccountPath  = getEnv("SERVICE_ACCOUNT_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token")
)

// CreateClient creates a new Vault API client.
func CreateClient() error {
	var err error
	vaultToken := os.Getenv("VAULT_TOKEN")

	config := &api.Config{
		Address: vaultAddress,
		HttpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	client, err = api.NewClient(config)
	if err != nil {
		return err
	}

	// Check which authentication method should be used.
	if vaultToken != "" {
		// Set the token, which should be used for the interaction with Vault.
		client.SetToken(vaultToken)
	} else {
		// Check the required mount path and role for the Kubernetes Auth
		// Method. If one of the env variable is missing we return an error.
		// if vaultKubernetesPath == "" {
		// return ErrMissingVaultKubernetesPath
		// }

		// if vaultKubernetesRole == "" {
		// return ErrMissingVaultKubernetesRole
		// }

		// Read the service account token value and create a map for the
		// authentication against Vault.
		// kubeToken, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		// if err != nil {
		// return err
		// }

		// data := make(map[string]interface{})
		// data["jwt"] = string(kubeToken)
		// data["role"] = vaultKubernetesRole

		// Authenticate against vault using the Kubernetes Auth Method and set
		// the token which the client should use for further interactions with
		// Vault. We also set the lease duration of the token for the renew
		// function.
		// secret, err := client.Logical().Write(vaultKubernetesPath+"/login", data)
		// if err != nil {
		// return err
		// } else if secret.Auth == nil {
		// return ErrMissingVaultAuthInfo
		// }

		// tokenLeaseDuration = secret.Auth.LeaseDuration

		// Read the JWT token from disk
		jwt, err := readJwtToken(serviceAccountPath)
		if err != nil {
			return err
		}

		// Authenticate to vault using the jwt token
		vaultToken, tokenLeaseDuration, err = authenticate(vaultKubernetesRole, jwt)
		if err != nil {
			return err
		}
		client.SetToken(vaultToken)
	}

	return nil
}

// RenewToken renews the provided token after the half of the lease duration is
// passed.
func RenewToken() {
	for {
		logger.Info("Renew Vault token")

		_, err := client.Auth().Token().RenewSelf(tokenLeaseDuration)
		if err != nil {
			logger.Error("Could not renew token: %s", err.Error())
		}

		time.Sleep(time.Duration(float64(tokenLeaseDuration)*0.5) * time.Second)
	}
}

// Encrypt uses vault transit engine
func Encrypt(d []byte) ([]byte, error) {
	s := transit.NewSeal(logger)
	config := map[string]string{
		"address":    client.Address(),
		"key_name":   vaultTransitKey,
		"token":      client.Token(),
		"mount_path": vaultTransitPath,
	}
	s.SetConfig(config)

	swi, err := s.Encrypt(context.Background(), d)
	if err != nil {
		return []byte{}, err
	}
	return swi.GetCiphertext(), nil
}

// Decrypt uses vault transit engine
func Decrypt(e []byte) ([]byte, error) {
	s := transit.NewSeal(logger)
	config := map[string]string{
		"address":    client.Address(),
		"key_name":   vaultTransitKey,
		"token":      client.Token(),
		"mount_path": vaultTransitPath,
	}
	s.SetConfig(config)

	data := &physical.EncryptedBlobInfo{
		Ciphertext: e,
	}
	return s.Decrypt(context.Background(), data)
}

// contains checks if a given key is in a slice of keys.
func contains(key string, keys []string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}

	return false
}

// getEnv looksup key, fallback on 2nd param.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func readJwtToken(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read jwt token: %s", err.Error())
	}

	return string(bytes.TrimSpace(data)), nil
}

func authenticate(role, jwt string) (string, int, error) {
	tlsClientConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsClientConfig,
	}

	client := &http.Client{
		Transport: transport,
	}

	transport.Proxy = http.ProxyFromEnvironment

	addr := vaultAddress + "/v1/auth/" + vaultKubernetesPath + "/login"
	body := fmt.Sprintf(`{"role": "%s", "jwt": "%s"}`, role, jwt)

	req, err := http.NewRequest(http.MethodPost, addr, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return "", 0, fmt.Errorf("failed to send request: %s", err.Error())
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to login: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var b bytes.Buffer
		if _, err := io.Copy(&b, resp.Body); err != nil {
			logger.Info("failed to copy response body: %s", err)
		}
		return "", 0, fmt.Errorf("failed to get successful response: %#v, %s",
			resp, b.String())
	}

	var s struct {
		Auth struct {
			ClientToken        string `json:"client_token"`
			TokenLeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return "", 0, fmt.Errorf("failed to decode message: %s", err.Error())
	}

	return s.Auth.ClientToken, s.Auth.TokenLeaseDuration, nil
}
