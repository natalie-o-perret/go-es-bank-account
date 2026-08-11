package app_test

import (
	"testing"
	"time"

	"github.com/natalie-o-perret/go-es-bank-account/app"
	"github.com/natalie-o-perret/go-es-bank-account/bank"
	"github.com/natalie-o-perret/go-functionalish/option"
)

var testTime = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

func TestStoreProjectsTransactions(t *testing.T) {
	store := app.NewStore()
	if outcome := store.Execute("checking", bank.Deposit(1000, testTime)); outcome.IsErr() {
		t.Fatalf("deposit failed: %v", outcome.UnwrapErr())
	}
	if outcome := store.Execute("checking", bank.Withdraw(500, testTime, option.None[bank.Money]())); outcome.IsErr() {
		t.Fatalf("withdraw failed: %v", outcome.UnwrapErr())
	}

	account := store.Account("checking")
	if account.Balance != 500 || account.Closed || len(account.Transactions) != 2 {
		t.Fatalf("unexpected account: %#v", account)
	}
	if account.Transactions[0].Amount != -500 || account.Transactions[0].Balance != 500 {
		t.Fatalf("unexpected latest transaction: %#v", account.Transactions[0])
	}
	if events := store.Events("checking"); len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
}

func TestStoreClosesAndDrainsPositiveBalance(t *testing.T) {
	store := app.NewStore()
	store.Execute("savings", bank.Deposit(1000, testTime))
	outcome := store.Execute("savings", bank.Close(testTime))
	if outcome.IsErr() {
		t.Fatalf("close failed: %v", outcome.UnwrapErr())
	}
	execution := outcome.Unwrap()
	if len(execution.Commit.Events) != 2 || execution.Commit.Events[0].Type != bank.Withdrawn || execution.Commit.Events[1].Type != bank.Closed {
		t.Fatalf("unexpected close commit: %#v", execution.Commit)
	}
	if !execution.Account.Closed || execution.Account.Balance != 0 || execution.Account.Transactions[0].Amount != -1000 {
		t.Fatalf("unexpected closed account: %#v", execution.Account)
	}
}

func TestRejectedCommandDoesNotAppend(t *testing.T) {
	store := app.NewStore()
	store.Execute("overdrawn", bank.Withdraw(500, testTime, option.None[bank.Money]()))
	before := len(store.Events("overdrawn"))
	outcome := store.Execute("overdrawn", bank.Close(testTime))
	if !outcome.IsErr() || outcome.UnwrapErr().Kind != bank.BalanceIsNegative {
		t.Fatalf("unexpected close outcome: %#v", outcome)
	}
	if after := len(store.Events("overdrawn")); after != before {
		t.Fatalf("event count changed from %d to %d", before, after)
	}
}

func TestClosingUnseenAccountCreatesProjection(t *testing.T) {
	store := app.NewStore()
	outcome := store.Execute("empty", bank.Close(testTime))
	if outcome.IsErr() {
		t.Fatalf("close failed: %v", outcome.UnwrapErr())
	}
	account := store.Account("empty")
	if !account.Closed || account.Balance != 0 || len(account.Transactions) != 0 {
		t.Fatalf("unexpected account: %#v", account)
	}
}

func TestAccountsAreSorted(t *testing.T) {
	store := app.NewStore()
	store.Execute("zebra", bank.Deposit(100, testTime))
	store.Execute("alpha", bank.Deposit(100, testTime))
	accounts := store.Accounts()
	if len(accounts) != 2 || accounts[0].ID != "alpha" || accounts[1].ID != "zebra" {
		t.Fatalf("unexpected account order: %#v", accounts)
	}
}
