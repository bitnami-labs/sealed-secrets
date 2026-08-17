package controller

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	certUtil "k8s.io/client-go/util/cert"
)

type testCertStore struct {
	sync.Mutex
	cert *x509.Certificate
}

func (c *testCertStore) getCert() ([]*x509.Certificate, error) {
	c.Lock()
	defer c.Unlock()
	return []*x509.Certificate{c.cert}, nil
}

func (c *testCertStore) setCert(cert *x509.Certificate) {
	c.Lock()
	defer c.Unlock()
	c.cert = cert
}

func shutdownServer(server *http.Server, t *testing.T) {
	err := server.Shutdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

// waitForServer polls addr until the server is accepting TCP connections,
// failing the test if it is not ready within a short deadline. This replaces
// a fixed sleep so the test proceeds as soon as the server is up and does not
// flake under load when startup takes longer than the sleep. It dials the
// socket rather than issuing an HTTP request so it does not exercise the
// handler before the test has populated the cert store.
func waitForServer(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start listening on %s: %v", addr, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestHttpCert(t *testing.T) {
	validFor := time.Hour
	cn := "my-cn"
	_, certBefore, err := generatePrivateKeyAndCert(2048, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}

	_, certAfter, err := generatePrivateKeyAndCert(2048, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}

	cs := &testCertStore{}
	server := httpserver(cs.getCert, nil, nil, 2, 2, nil)
	defer shutdownServer(server, t)
	hp := *listenAddr
	if strings.HasPrefix(hp, ":") {
		hp = fmt.Sprintf("localhost%s", hp)
	}

	// Wait for the server to accept connections instead of sleeping a fixed
	// amount of time and hoping it is ready.
	waitForServer(t, hp)

	check := func(cert *x509.Certificate) {
		resp, err := http.Get(fmt.Sprintf("http://%s/v1/cert.pem", hp))
		if err != nil {
			t.Fatal(err)
		}

		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Fatalf("got: %v, want: %v", got, want)
		}
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		certs, err := certUtil.ParseCertsPEM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(certs), 1; got != want {
			t.Fatalf("got: %v, want: %v", got, want)
		}
		if got, want := certs[0], cert; !got.Equal(want) {
			t.Fatalf("got: %v, want: %v", got, want)
		}
	}

	cs.setCert(certBefore)
	check(certBefore)

	cs.setCert(certAfter)
	check(certAfter)
}

func TestHttpReadyz(t *testing.T) {
	ready := false
	server := httpserver(func() ([]*x509.Certificate, error) {
		return nil, fmt.Errorf("no cert")
	}, nil, nil, 2, 2, func() bool { return ready })
	defer shutdownServer(server, t)

	hp := *listenAddr
	if strings.HasPrefix(hp, ":") {
		hp = fmt.Sprintf("localhost%s", hp)
	}
	url := fmt.Sprintf("http://%s/readyz", hp)
	healthURL := fmt.Sprintf("http://%s/healthz", hp)

	// Wait for listener
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not become reachable")
		}
		time.Sleep(50 * time.Millisecond)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("before ready: got status %v want %v body %q", got, want, body)
	}

	ready = true
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("after ready: got status %v want %v body %q", got, want, body)
	}
}
