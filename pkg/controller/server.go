package controller

import (
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	flag "github.com/spf13/pflag"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
	certUtil "k8s.io/client-go/util/cert"
)

var (
	listenAddr        = flag.String("listen-addr", ":8080", "HTTP serving address.")
	listenMetricsAddr = flag.String("listen-metrics-addr", ":8081", "HTTP metrics serving address.")
	readTimeout       = flag.Duration("read-timeout", 2*time.Minute, "HTTP request timeout.")
	writeTimeout      = flag.Duration("write-timeout", 2*time.Minute, "HTTP response timeout.")
)

// Called on every request to /cert.  Errors will be logged and return a 500.
type certProvider func() ([]*x509.Certificate, error)
type secretChecker func([]byte) (bool, error)
type secretRotator func([]byte) ([]byte, error)

// httpserver starts an HTTP that exposes core functionality like serving the public key
// or secret rotation and validation. This endpoint is designed to be accessible by
// all users of a given cluster. It must not leak any secret material.
// The server is started in the background and a handle to it returned so it can be shut down.
func httpserver(cp certProvider, sc secretChecker, sr secretRotator, burst int, rate int) *http.Server {
	httpRateLimiter := rateLimiter(burst, rate)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err := io.WriteString(w, "ok\n")
		if err != nil {
			log.Fatal(err)
		}
	})

	mux.Handle("/v1/verify", Instrument("/v1/verify", httpRateLimiter.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("Error handling /v1/verify request", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		valid, err := sc(content)
		if err != nil {
			slog.Error("Error validating secret", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if valid {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusConflict)
		}
	}))))

	// TODO(mkm): rename to re-encrypt
	mux.Handle("/v1/rotate", Instrument("/v1/rotate", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("Error handling /v1/rotate request", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		newSecret, err := sr(content)
		if err != nil {
			slog.Error("Error rotating secret", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(newSecret)
	})))

	mux.Handle("/v1/cert.pem", Instrument("/v1/cert.pem", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		certs, err := cp()
		if err != nil {
			slog.Error("cannot get certificates", "error", err)
			http.Error(w, "cannot get certificate", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-pem-file")
		for _, cert := range certs {
			_, _ = w.Write(pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}))
		}
	})))

	server := http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadTimeout:       *readTimeout,
		ReadHeaderTimeout: *readTimeout,
		WriteTimeout:      *writeTimeout,
	}

	slog.Info("HTTP server serving", "addr", server.Addr)
	go func() {
		err := server.ListenAndServe()
		slog.Error("HTTP server exiting", "error", err)
	}()
	return &server
}

func httpserverMetrics() *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := http.Server{
		Addr:              *listenMetricsAddr,
		Handler:           mux,
		ReadTimeout:       *readTimeout,
		ReadHeaderTimeout: *readTimeout,
		WriteTimeout:      *writeTimeout,
	}

	slog.Info("HTTP metrics server serving", "addr", server.Addr)
	go func() {
		err := server.ListenAndServe()
		slog.Error("HTTP metrics server exiting", "error", err)
	}()
	return &server
}

func rateLimiter(burst int, rate int) throttled.HTTPRateLimiter {
	store, err := memstore.New(65536)
	if err != nil {
		log.Fatal(err)
	}

	quota := throttled.RateQuota{MaxRate: throttled.PerSec(rate), MaxBurst: burst}
	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		log.Fatal(err)
	}
	return throttled.HTTPRateLimiter{
		RateLimiter: rateLimiter,
		VaryBy:      &throttled.VaryBy{Path: true, Headers: []string{"X-Forwarded-For"}},
	}
}
