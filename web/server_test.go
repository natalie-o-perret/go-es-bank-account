package web_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natalie-o-perret/go-es-bank-account/app"
	"github.com/natalie-o-perret/go-es-bank-account/web"
)

var testTime = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

func TestWebApplicationAndCommandFlow(t *testing.T) {
	server := httptest.NewServer(web.New(app.NewStore(), func() time.Time { return testTime }))
	defer server.Close()

	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Event Ledger")) {
		t.Fatalf("unexpected root response: status=%d err=%v", response.StatusCode, err)
	}

	postCommand(t, server.URL, `{"accountId":"checking","type":"deposit","amount":"10.00"}`, http.StatusOK)
	postCommand(t, server.URL, `{"accountId":"checking","type":"withdraw","amount":"4.25"}`, http.StatusOK)

	response, err = http.Get(server.URL + "/api/account?id=checking")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var account struct {
		Balance      string `json:"balance"`
		Transactions []struct {
			Amount  string `json:"amount"`
			Balance string `json:"balance"`
		} `json:"transactions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&account); err != nil {
		t.Fatal(err)
	}
	if account.Balance != "5.75" || len(account.Transactions) != 2 || account.Transactions[0].Amount != "-4.25" {
		t.Fatalf("unexpected account response: %#v", account)
	}
}

func TestCommandValidationAndDomainErrors(t *testing.T) {
	server := httptest.NewServer(web.New(app.NewStore(), func() time.Time { return testTime }))
	defer server.Close()

	postCommand(t, server.URL, `{"accountId":"checking","type":"deposit","amount":"0"}`, http.StatusBadRequest)
	postCommand(t, server.URL, `{"accountId":"checking","type":"withdraw","amount":"5.00"}`, http.StatusOK)
	body := postCommand(t, server.URL, `{"accountId":"checking","type":"close"}`, http.StatusConflict)
	if !strings.Contains(body, "Balance is negative, closing denied") {
		t.Fatalf("unexpected close error: %s", body)
	}
}

func TestCommandRejectsOverflowAndUnsafeInput(t *testing.T) {
	server := httptest.NewServer(web.New(app.NewStore(), func() time.Time { return testTime }))
	defer server.Close()

	postCommand(t, server.URL, `{"accountId":"checking","type":"deposit","amount":"92233720368547758.07"}`, http.StatusOK)
	body := postCommand(t, server.URL, `{"accountId":"checking","type":"deposit","amount":"0.01"}`, http.StatusConflict)
	if !strings.Contains(body, "Balance overflow, transaction denied") {
		t.Fatalf("unexpected overflow error: %s", body)
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/commands", strings.NewReader(`{"accountId":"checking","type":"deposit","amount":"1.00"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "text/plain")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain status = %d, want %d", response.StatusCode, http.StatusUnsupportedMediaType)
	}

	longID := strings.Repeat("x", 81)
	postCommand(t, server.URL, `{"accountId":"`+longID+`","type":"deposit","amount":"1.00"}`, http.StatusBadRequest)
	postCommand(t, server.URL, `{"accountId":"zero\u200bwidth","type":"deposit","amount":"1.00"}`, http.StatusBadRequest)
}

func postCommand(t *testing.T, baseURL, body string, wantStatus int) string {
	t.Helper()
	response, err := http.Post(baseURL+"/api/commands", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("POST /api/commands: got %d %s, want %d", response.StatusCode, data, wantStatus)
	}
	return string(data)
}
