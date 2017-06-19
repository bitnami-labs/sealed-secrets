package main

import (
	"crypto/x509"
	"flag"
	"io"
	"log"
	"net/http"
	"time"

	certUtil "k8s.io/client-go/util/cert"
)

var (
	listenAddr   = flag.String("listen-addr", ":8080", "HTTP serving address.")
	readTimeout  = flag.Duration("read-timeout", 2*time.Minute, "HTTP request timeout.")
	writeTimeout = flag.Duration("write-timeout", 2*time.Minute, "HTTP response timeout.")
)

// Called on every request to /cert.  Errors will be logged and return a 500.
type certProvider func() ([]*x509.Certificate, error)

func httpserver(cp certProvider) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "ok\n")
	})

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
