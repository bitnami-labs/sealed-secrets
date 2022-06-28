package kubeseal

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

func IsFilename(name string) (bool, error) {
	u, err := url.Parse(name)
	if err != nil {
		return false, err
	}
	// windows drive letters
	if s := strings.ToLower(u.Scheme); len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
		return true, nil
	}
	return u.Scheme == "", nil
}

// openCertLocal opens a cert URI or local filename, by fetching it locally from the client
// (as opposed as openCertCluster which fetches it via HTTP but through the k8s API proxy).
func OpenCertLocal(filenameOrURI string) (io.ReadCloser, error) {
	// detect if a certificate is a local file or an URI.
	if ok, err := IsFilename(filenameOrURI); err != nil {
		return nil, err
	} else if ok {
		// #nosec G304 -- should open user provided file
		return os.Open(filenameOrURI)
	}
	return OpenCertURI(filenameOrURI)
}

func OpenCertURI(uri string) (io.ReadCloser, error) {
	// support file:// scheme. Note: we're opening the file using os.Open rather
	// than using the file:// scheme below because there is no point in complicating our lives
	// and escape the filename properly.

	t := &http.Transport{}
	// #nosec: G111 -- we want to allow all files to be opened
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	c := &http.Client{Transport: t}

	resp, err := c.Get(uri)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch %q: %s", uri, resp.Status)
	}
	return resp.Body, nil
}

// openCertCluster fetches a certificate by performing an HTTP request to the controller
// through the k8s API proxy.
func OpenCertCluster(ctx context.Context, c corev1.CoreV1Interface, namespace, name string) (io.ReadCloser, error) {
	portName, err := GetServicePortName(ctx, c, namespace, name)
	if err != nil {
		return nil, err
	}
	cert, err := c.Services(namespace).ProxyGet("http", name, portName, "/v1/cert.pem", nil).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	return cert, nil
}

func OpenCert(ctx context.Context, clientConfig clientcmd.ClientConfig, certURL, controllerNS, controllerName string) (io.ReadCloser, error) {
	if certURL != "" {
		return OpenCertLocal(certURL)
	}

	restConfig, err := clientConfig.ClientConfig()

	if err != nil {
		return nil, err
	}

	restConfig.AcceptContentTypes = "application/x-pem-file, */*"
	restClient, err := corev1.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return OpenCertCluster(ctx, restClient, controllerNS, controllerName)
}

func ParseKey(r io.Reader) (*rsa.PublicKey, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	certs, err := cert.ParseCertsPEM(data)
	if err != nil {
		return nil, err
	}

	// ParseCertsPem returns error if len(certs) == 0, but best to be sure...
	if len(certs) == 0 {
		return nil, errors.New("Failed to read any certificates")
	}

	cert, ok := certs[0].PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Expected RSA public key but found %v", certs[0].PublicKey)
	}

	if time.Now().After(certs[0].NotAfter) {
		return nil, fmt.Errorf("failed to encrypt using an expired certificate on %v", certs[0].NotBefore.Format("January 2, 2006"))
	}

	return cert, nil
}
