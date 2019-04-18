package main

import (
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
	certUtil "k8s.io/client-go/util/cert"
)

var (
	listenAddr   = flag.String("listen-addr", ":8080", "HTTP serving address.")
	readTimeout  = flag.Duration("read-timeout", 2*time.Minute, "HTTP request timeout.")
	writeTimeout = flag.Duration("write-timeout", 2*time.Minute, "HTTP response timeout.")
)

// Called on every request to /cert.  Errors will be logged and return a 500.
type certProvider func() ([]*x509.Certificate, error)

type secretChecker func([]byte) (bool, error)

func httpserver(cp certProvider, sc secretChecker) {
	httpRateLimiter := rateLimter()

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "ok\n")
	})

	mux.Handle("/v1/verify", httpRateLimiter.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, err := ioutil.ReadAll(r.Body)

		if err != nil {
			log.Printf("Error handling /v1/verify request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		valid, err := sc(content)

		if err != nil {
			log.Printf("Error validating secret: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if valid {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusConflict)
		}

	})))

	mux.HandleFunc("/v1/cert.pem", func(w http.ResponseWriter, r *http.Request) {
		certs, err := cp()

		if err != nil {
			log.Printf("Error handling /cert request: %v", err)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "Internal error\n")
			return
		}

		w.Header().Set("Content-Type", "application/x-pem-file")
		for _, cert := range certs {
			w.Write(certUtil.EncodeCertPEM(cert))
		}
	})

	server := http.Server{
		Addr:         *listenAddr,
		Handler:      mux,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}

	log.Printf("HTTP server serving on %s", server.Addr)
	err := server.ListenAndServe()
	log.Printf("HTTP server exiting: %v", err)
}

func rateLimter() throttled.HTTPRateLimiter {
	store, err := memstore.New(65536)
	if err != nil {
		log.Fatal(err)
	}

	quota := throttled.RateQuota{MaxRate: throttled.PerSec(2), MaxBurst: 2}
	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		log.Fatal(err)
	}
	return throttled.HTTPRateLimiter{
		RateLimiter: rateLimiter,
		VaryBy:      &throttled.VaryBy{Path: true, Headers: []string{"X-Forwarded-For"}},
	}

}
