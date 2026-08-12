// Package client exposes the bank-account application over HTTP.
package client

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/natalie-o-perret/go-es-bank-account/features/bank/app"
	bank "github.com/natalie-o-perret/go-es-bank-account/features/bank/domain"
	"github.com/natalie-o-perret/go-functionalish/option"
	"github.com/natalie-o-perret/go-functionalish/slice"
)

//go:embed static/*
var staticFiles embed.FS

type server struct {
	store *app.Store
	now   func() time.Time
}

// New returns the complete API and static web application.
func New(store *app.Store, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	public, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}

	s := server{store: store, now: now}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/accounts", s.getAccounts)
	mux.HandleFunc("GET /api/account", s.getAccount)
	mux.HandleFunc("GET /api/commits", s.getCommits)
	mux.HandleFunc("POST /api/commands", s.postCommand)
	mux.Handle("GET /", http.FileServer(http.FS(public)))
	return mux
}

type transactionResponse struct {
	At      string `json:"at"`
	Amount  string `json:"amount"`
	Balance string `json:"balance"`
}

type accountResponse struct {
	ID           string                `json:"id"`
	Balance      string                `json:"balance"`
	Closed       bool                  `json:"closed"`
	Transactions []transactionResponse `json:"transactions"`
}

type accountSummaryResponse struct {
	ID      string `json:"id"`
	Balance string `json:"balance"`
	Closed  bool   `json:"closed"`
}

type eventResponse struct {
	Type   bank.EventType `json:"type"`
	Amount string         `json:"amount,omitempty"`
	At     string         `json:"at"`
}

type commitResponse struct {
	Stream string          `json:"stream"`
	Events []eventResponse `json:"events"`
}

type commandRequest struct {
	AccountID string `json:"accountId"`
	Type      string `json:"type"`
	Amount    string `json:"amount"`
}

type commandResponse struct {
	Account accountResponse `json:"account"`
	Commit  commitResponse  `json:"commit"`
}

func (s server) getAccounts(w http.ResponseWriter, _ *http.Request) {
	response := slice.FromSlice(s.store.Accounts()).Map(func(account app.AccountSummary) accountSummaryResponse {
		return accountSummaryResponse{
			ID: account.ID, Balance: account.Balance.String(), Closed: account.Closed,
		}
	}).ToSlice()
	writeJSON(w, http.StatusOK, response)
}

func (s server) getAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("id")
	if !validAccountID(accountID) {
		writeError(w, http.StatusBadRequest, "account ID must be 1 to 80 printable characters")
		return
	}
	writeJSON(w, http.StatusOK, accountToResponse(s.store.Account(accountID)))
}

func (s server) getCommits(w http.ResponseWriter, _ *http.Request) {
	response := slice.FromSlice(s.store.Commits()).Map(commitToResponse).ToSlice()
	writeJSON(w, http.StatusOK, response)
}

func (s server) postCommand(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request commandRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	if !validAccountID(request.AccountID) {
		writeError(w, http.StatusBadRequest, "account ID must be 1 to 80 printable characters")
		return
	}

	at := s.now().UTC()
	var command bank.Command
	switch strings.ToLower(request.Type) {
	case "deposit":
		amount, ok := positiveAmount(w, request.Amount)
		if !ok {
			return
		}
		command = bank.Deposit(amount, at)
	case "withdraw":
		amount, ok := positiveAmount(w, request.Amount)
		if !ok {
			return
		}
		command = bank.Withdraw(amount, at, option.None[bank.Money]())
	case "close":
		command = bank.Close(at)
	default:
		writeError(w, http.StatusBadRequest, "unknown command type")
		return
	}

	outcome := s.store.Execute(request.AccountID, command)
	if outcome.IsErr() {
		writeError(w, http.StatusConflict, outcome.UnwrapErr().Error())
		return
	}
	execution := outcome.Unwrap()
	writeJSON(w, http.StatusOK, commandResponse{
		Account: accountToResponse(execution.Account),
		Commit:  commitToResponse(execution.Commit),
	})
}

func positiveAmount(w http.ResponseWriter, input string) (bank.Money, bool) {
	amount, err := bank.ParseMoney(input)
	if err != nil || amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than zero with at most two decimal places")
		return 0, false
	}
	return amount, true
}

func validAccountID(accountID string) bool {
	if strings.TrimSpace(accountID) == "" || utf8.RuneCountInString(accountID) > 80 {
		return false
	}
	return strings.IndexFunc(accountID, func(r rune) bool { return !unicode.IsPrint(r) }) == -1
}

func accountToResponse(account app.Account) accountResponse {
	transactions := slice.FromSlice(account.Transactions).Map(func(transaction app.Transaction) transactionResponse {
		return transactionResponse{
			At:      transaction.At.Format(time.RFC3339Nano),
			Amount:  transaction.Amount.String(),
			Balance: transaction.Balance.String(),
		}
	}).ToSlice()
	return accountResponse{
		ID:           account.ID,
		Balance:      account.Balance.String(),
		Closed:       account.Closed,
		Transactions: transactions,
	}
}

func commitToResponse(commit app.Commit) commitResponse {
	events := slice.FromSlice(commit.Events).Map(func(event bank.Event) eventResponse {
		response := eventResponse{
			Type: event.Type,
			At:   event.At.Format(time.RFC3339Nano),
		}
		if event.Type != bank.Closed {
			response.Amount = event.Amount.String()
		}
		return response
	}).ToSlice()
	return commitResponse{Stream: commit.Stream, Events: events}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
