package domain

import (
	"github.com/natalie-o-perret/go-functionalish/result"
	"github.com/natalie-o-perret/go-functionalish/seq"
)

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
