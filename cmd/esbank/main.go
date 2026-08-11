package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/natalie-o-perret/go-es-bank-account/app"
	"github.com/natalie-o-perret/go-es-bank-account/web"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	flag.Parse()

	server := &http.Server{
		Addr:              *address,
		Handler:           hostGuard(web.New(app.NewStore(), time.Now), *address),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("event-sourced bank account listening on http://%s", *address)
	log.Fatal(server.ListenAndServe())
}

func hostGuard(next http.Handler, address string) http.Handler {
	listenHost, _, _ := net.SplitHostPort(address)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHost := r.Host
		if host, _, err := net.SplitHostPort(requestHost); err == nil {
			requestHost = host
		}
		requestHost = strings.Trim(requestHost, "[]")
		allowed := net.ParseIP(requestHost) != nil ||
			strings.EqualFold(requestHost, "localhost") ||
			listenHost != "" && strings.EqualFold(requestHost, listenHost)
		if !allowed {
			http.Error(w, "invalid Host header", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
