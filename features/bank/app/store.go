// Package app coordinates event streams and the account read model.
package app

import (
	"cmp"
	"slices"
	"sync"
	"time"

	bank "github.com/natalie-o-perret/go-es-bank-account/features/bank/domain"
	"github.com/natalie-o-perret/go-functionalish/result"
)

// Transaction is a signed ledger row with its resulting balance.
type Transaction struct {
	At      time.Time
	Amount  bank.Money
	Balance bank.Money
}

// Account is the projected account view.
type Account struct {
	ID           string
	Balance      bank.Money
	Closed       bool
	Transactions []Transaction
}

// AccountSummary is the home-page account view.
type AccountSummary struct {
	ID      string
	Balance bank.Money
	Closed  bool
}

// Commit groups the events emitted atomically by one command.
type Commit struct {
	Stream string
	Events []bank.Event
}

// Execution is the result of a successful command.
type Execution struct {
	Account Account
	Commit  Commit
}

// Store is a process-local event store with an atomically updated read model.
type Store struct {
	// ponytail: one lock is enough for this local kata; split by stream if throughput matters.
	mu       sync.RWMutex
	streams  map[string][]bank.Event
	accounts map[string]Account
	commits  []Commit
}

// NewStore creates an empty volatile store.
func NewStore() *Store {
	return &Store{
		streams:  make(map[string][]bank.Event),
		accounts: make(map[string]Account),
	}
}

// Execute folds history, decides, appends, and projects under one lock.
func (s *Store) Execute(accountID string, command bank.Command) result.Result[Execution, bank.DomainError] {
	s.mu.Lock()
	defer s.mu.Unlock()

	initial := bank.Fold(s.streams[accountID])
	decision := bank.Decide(command, initial)
	if decision.IsErr() {
		return result.Err[Execution, bank.DomainError](decision.UnwrapErr())
	}

	events := append([]bank.Event(nil), decision.Unwrap()...)
	s.streams[accountID] = append(s.streams[accountID], events...)

	account, exists := s.accounts[accountID]
	if !exists {
		account = Account{ID: accountID}
	}
	for _, pair := range bank.EvolveWithStates(initial, events) {
		account.Balance = pair.State.Balance
		account.Closed = pair.State.Closed
		switch pair.Event.Type {
		case bank.Deposited:
			account.Transactions = prepend(account.Transactions, Transaction{
				At: pair.Event.At, Amount: pair.Event.Amount, Balance: pair.State.Balance,
			})
		case bank.Withdrawn:
			account.Transactions = prepend(account.Transactions, Transaction{
				At: pair.Event.At, Amount: -pair.Event.Amount, Balance: pair.State.Balance,
			})
		case bank.Closed:
		}
	}
	s.accounts[accountID] = account

	commit := Commit{Stream: "BankAccount-" + accountID, Events: events}
	s.commits = append([]Commit{commit}, s.commits...)
	return result.Ok[Execution, bank.DomainError](Execution{
		Account: copyAccount(account),
		Commit:  copyCommit(commit),
	})
}

// Account returns an account projection. An unseen account is open and empty.
func (s *Store) Account(accountID string) Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, exists := s.accounts[accountID]
	if !exists {
		return Account{ID: accountID}
	}
	return copyAccount(account)
}

// Accounts returns all projected accounts sorted by ID.
func (s *Store) Accounts() []AccountSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summaries := make([]AccountSummary, 0, len(s.accounts))
	for _, account := range s.accounts {
		summaries = append(summaries, AccountSummary{
			ID: account.ID, Balance: account.Balance, Closed: account.Closed,
		})
	}
	slices.SortFunc(summaries, func(a, b AccountSummary) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return summaries
}

// Events returns a copy of one account's source-of-truth history.
func (s *Store) Events(accountID string) []bank.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]bank.Event(nil), s.streams[accountID]...)
}

// Commits returns all command commits, newest first.
func (s *Store) Commits() []Commit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	commits := make([]Commit, len(s.commits))
	for i, commit := range s.commits {
		commits[i] = copyCommit(commit)
	}
	return commits
}

func prepend(items []Transaction, item Transaction) []Transaction {
	result := make([]Transaction, len(items)+1)
	result[0] = item
	copy(result[1:], items)
	return result
}

func copyAccount(account Account) Account {
	account.Transactions = append([]Transaction(nil), account.Transactions...)
	return account
}

func copyCommit(commit Commit) Commit {
	commit.Events = append([]bank.Event(nil), commit.Events...)
	return commit
}
