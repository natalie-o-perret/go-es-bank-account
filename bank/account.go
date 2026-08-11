// Package bank contains the pure bank-account decider.
package bank

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/natalie-o-perret/go-functionalish/option"
	"github.com/natalie-o-perret/go-functionalish/result"
	"github.com/natalie-o-perret/go-functionalish/seq"
)

// Money stores an exact amount in cents.
type Money int64

// ErrInvalidMoney reports an amount that is not a decimal with at most two
// fractional digits or that exceeds the int64 cent range.
var ErrInvalidMoney = errors.New("invalid money amount")

// ParseMoney parses a base-10 amount into cents.
func ParseMoney(input string) (Money, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, ErrInvalidMoney
	}

	negative := false
	switch value[0] {
	case '-':
		negative = true
		value = value[1:]
	case '+':
		value = value[1:]
	}
	if value == "" {
		return 0, ErrInvalidMoney
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidMoney
	}
	dollars, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidMoney
	}

	var cents uint64
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, ErrInvalidMoney
		}
		cents, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return 0, ErrInvalidMoney
		}
		if len(parts[1]) == 1 {
			cents *= 10
		}
	}

	limit := uint64(1<<63 - 1)
	if negative {
		limit = uint64(1 << 63)
	}
	if cents > limit || dollars > (limit-cents)/100 {
		return 0, ErrInvalidMoney
	}
	total := dollars*100 + cents
	if !negative {
		return Money(total), nil
	}
	if total == uint64(1<<63) {
		return Money(-1 << 63), nil
	}
	return -Money(total), nil
}

func (m Money) String() string {
	negative := m < 0
	var value uint64
	if negative {
		value = uint64(-(m + 1)) + 1
	} else {
		value = uint64(m)
	}
	prefix := ""
	if negative {
		prefix = "-"
	}
	return fmt.Sprintf("%s%d.%02d", prefix, value/100, value%100)
}

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

// Evolve applies an event without validating history.
func Evolve(state State, event Event) State {
	switch event.Type {
	case Deposited:
		balance, ok := checkedAdd(state.Balance, event.Amount)
		if !ok {
			panic("bank: balance overflow")
		}
		state.Balance = balance
	case Withdrawn:
		balance, ok := checkedSubtract(state.Balance, event.Amount)
		if !ok {
			panic("bank: balance overflow")
		}
		state.Balance = balance
	case Closed:
		state.Closed = true
	default:
		panic("bank: unknown event type")
	}
	return state
}

// Fold reconstructs state from event history.
func Fold(events []Event) State {
	return seq.OfSlice(events).Fold(Initial(), Evolve)
}

// EventState pairs an event with its post-event state.
type EventState struct {
	Event Event
	State State
}

// EvolveWithStates applies events in order and returns each post-event state.
func EvolveWithStates(initial State, events []Event) []EventState {
	pairs := make([]EventState, 0, len(events))
	state := initial
	for _, event := range events {
		state = Evolve(state, event)
		pairs = append(pairs, EventState{Event: event, State: state})
	}
	return pairs
}

// Decide validates a command and returns the events it should emit.
func Decide(command Command, state State) result.Result[[]Event, DomainError] {
	if state.Closed {
		return result.Err[[]Event, DomainError](DomainError{Kind: AlreadyClosed})
	}

	switch command.Type {
	case DepositCommand:
		if _, ok := checkedAdd(state.Balance, command.Amount); !ok {
			return result.Err[[]Event, DomainError](DomainError{Kind: BalanceOverflow})
		}
		return result.Ok[[]Event, DomainError]([]Event{{
			Type: Deposited, Amount: command.Amount, At: command.At,
		}})
	case WithdrawCommand:
		if command.Amount == minMoney {
			return result.Err[[]Event, DomainError](DomainError{Kind: BalanceOverflow})
		}
		upcoming, ok := checkedSubtract(state.Balance, command.Amount)
		if !ok {
			return result.Err[[]Event, DomainError](DomainError{Kind: BalanceOverflow})
		}
		if command.Threshold.IsSome() {
			threshold := command.Threshold.Unwrap()
			if upcoming < threshold {
				return result.Err[[]Event, DomainError](DomainError{
					Kind:            ThresholdExceeded,
					UpcomingBalance: upcoming,
					ThresholdLimit:  threshold,
				})
			}
		}
		return result.Ok[[]Event, DomainError]([]Event{{
			Type: Withdrawn, Amount: command.Amount, At: command.At,
		}})
	case CloseCommand:
		if state.Balance < 0 {
			return result.Err[[]Event, DomainError](DomainError{
				Kind: BalanceIsNegative, Balance: state.Balance,
			})
		}
		events := make([]Event, 0, 2)
		if state.Balance > 0 {
			events = append(events, Event{
				Type: Withdrawn, Amount: state.Balance, At: command.At,
			})
		}
		events = append(events, Event{Type: Closed, At: command.At})
		return result.Ok[[]Event, DomainError](events)
	default:
		panic("bank: unknown command type")
	}
}

const (
	maxMoney = Money(1<<63 - 1)
	minMoney = Money(-1 << 63)
)

func checkedAdd(a, b Money) (Money, bool) {
	if b > 0 && a > maxMoney-b || b < 0 && a < minMoney-b {
		return 0, false
	}
	return a + b, true
}

func checkedSubtract(a, b Money) (Money, bool) {
	if b > 0 && a < minMoney+b || b < 0 && a > maxMoney+b {
		return 0, false
	}
	return a - b, true
}
