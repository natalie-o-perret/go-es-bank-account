// Package domain contains the pure bank-account decider.
package domain

import (
	"time"

	"github.com/natalie-o-perret/go-functionalish/option"
)

// State is reconstructed entirely from an account's events.
type State struct {
	Balance Money
	Closed  bool
}

// Initial returns the state of an unseen account.
func Initial() State { return State{} }

// EventType identifies a persisted domain event.
type EventType string

const (
	Deposited EventType = "Deposited"
	Withdrawn EventType = "Withdrawn"
	Closed    EventType = "Closed"
)

// Event records what happened to an account.
type Event struct {
	Type   EventType
	Amount Money
	At     time.Time
}

// CommandType identifies a requested operation.
type CommandType uint8

const (
	DepositCommand CommandType = iota + 1
	WithdrawCommand
	CloseCommand
)

// Command contains all data required by the pure decider.
type Command struct {
	Type      CommandType
	Amount    Money
	At        time.Time
	Threshold option.Option[Money]
}

// Deposit requests a deposit.
func Deposit(amount Money, at time.Time) Command {
	return Command{Type: DepositCommand, Amount: amount, At: at}
}

// Withdraw requests a withdrawal with an optional minimum balance.
func Withdraw(amount Money, at time.Time, threshold option.Option[Money]) Command {
	return Command{Type: WithdrawCommand, Amount: amount, At: at, Threshold: threshold}
}

// Close requests account closure.
func Close(at time.Time) Command { return Command{Type: CloseCommand, At: at} }

// ErrorKind identifies a business-rule rejection.
type ErrorKind string

const (
	AlreadyClosed     ErrorKind = "AlreadyClosed"
	ThresholdExceeded ErrorKind = "ThresholdExceeded"
	BalanceIsNegative ErrorKind = "BalanceIsNegative"
	BalanceOverflow   ErrorKind = "BalanceOverflow"
)

// DomainError contains the values used to reject a command.
type DomainError struct {
	Kind            ErrorKind
	Balance         Money
	UpcomingBalance Money
	ThresholdLimit  Money
}

func (e DomainError) Error() string {
	switch e.Kind {
	case AlreadyClosed:
		return "Account already closed"
	case ThresholdExceeded:
		return "Threshold exceeded, withdrawal denied"
	case BalanceIsNegative:
		return "Balance is negative, closing denied"
	case BalanceOverflow:
		return "Balance overflow, transaction denied"
	default:
		return "Unknown domain error"
	}
}
