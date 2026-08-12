package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/natalie-o-perret/go-functionalish/kv"
	"github.com/natalie-o-perret/go-functionalish/seq"
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
	kv.Of(tests).ToSeq().Iter(func(test seq.Pair[string, int]) {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)
		request.Host = test.First
		response := httptest.NewRecorder()
		hostGuard(next, "ledger:8080").ServeHTTP(response, request)
		if response.Code != test.Second {
			t.Errorf("Host %q: got %d, want %d", test.First, response.Code, test.Second)
		}
	})
}
