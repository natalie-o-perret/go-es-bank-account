package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostGuard(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	tests := map[string]int{
		"127.0.0.1:8080": http.StatusNoContent,
		"[::1]:8080":     http.StatusNoContent,
		"localhost:8080": http.StatusNoContent,
		"ledger:8080":    http.StatusNoContent,
		"attacker.test":  http.StatusMisdirectedRequest,
	}
	for host, want := range tests {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)
		request.Host = host
		response := httptest.NewRecorder()
		hostGuard(next, "ledger:8080").ServeHTTP(response, request)
		if response.Code != want {
			t.Errorf("Host %q: got %d, want %d", host, response.Code, want)
		}
	}
}
