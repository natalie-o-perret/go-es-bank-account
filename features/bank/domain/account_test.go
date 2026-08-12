package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	bank "github.com/natalie-o-perret/go-es-bank-account/features/bank/domain"
	"github.com/natalie-o-perret/go-functionalish/option"
)

var testTime = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

func TestDecideDepositAndWithdraw(t *testing.T) {
	deposit := bank.Decide(bank.Deposit(1000, testTime), bank.Initial())
	if deposit.IsErr() {
		t.Fatalf("deposit failed: %v", deposit.UnwrapErr())
	}
	wantDeposit := []bank.Event{{Type: bank.Deposited, Amount: 1000, At: testTime}}
	if got := deposit.Unwrap(); !reflect.DeepEqual(got, wantDeposit) {
		t.Fatalf("deposit events: got %#v, want %#v", got, wantDeposit)
	}

	withdraw := bank.Decide(bank.Withdraw(1000, testTime, option.None[bank.Money]()), bank.Initial())
	if withdraw.IsErr() {
		t.Fatalf("withdraw failed: %v", withdraw.UnwrapErr())
	}
	wantWithdraw := []bank.Event{{Type: bank.Withdrawn, Amount: 1000, At: testTime}}
	if got := withdraw.Unwrap(); !reflect.DeepEqual(got, wantWithdraw) {
		t.Fatalf("withdraw events: got %#v, want %#v", got, wantWithdraw)
	}
}

func TestWithdrawalThreshold(t *testing.T) {
	state := bank.Fold([]bank.Event{{Type: bank.Withdrawn, Amount: 40000, At: testTime}})
	allowed := bank.Decide(bank.Withdraw(10000, testTime, option.Some(bank.Money(-50000))), state)
	if allowed.IsErr() {
		t.Fatalf("withdrawal at threshold failed: %v", allowed.UnwrapErr())
	}
	state = bank.Evolve(state, allowed.Unwrap()[0])

	denied := bank.Decide(bank.Withdraw(100, testTime, option.Some(bank.Money(-50000))), state)
	if !denied.IsErr() {
		t.Fatal("withdrawal below threshold succeeded")
	}
	err := denied.UnwrapErr()
	if err.Kind != bank.ThresholdExceeded || err.UpcomingBalance != -50100 || err.ThresholdLimit != -50000 {
		t.Fatalf("unexpected threshold error: %#v", err)
	}
}

func TestClose(t *testing.T) {
	t.Run("zero balance", func(t *testing.T) {
		decision := bank.Decide(bank.Close(testTime), bank.Initial())
		want := []bank.Event{{Type: bank.Closed, At: testTime}}
		if decision.IsErr() || !reflect.DeepEqual(decision.Unwrap(), want) {
			t.Fatalf("got %#v, want %#v", decision, want)
		}
	})

	t.Run("positive balance", func(t *testing.T) {
		state := bank.Fold([]bank.Event{{Type: bank.Deposited, Amount: 10000, At: testTime}})
		decision := bank.Decide(bank.Close(testTime), state)
		want := []bank.Event{
			{Type: bank.Withdrawn, Amount: 10000, At: testTime},
			{Type: bank.Closed, At: testTime},
		}
		if decision.IsErr() || !reflect.DeepEqual(decision.Unwrap(), want) {
			t.Fatalf("got %#v, want %#v", decision, want)
		}
	})

	t.Run("negative balance", func(t *testing.T) {
		state := bank.Fold([]bank.Event{{Type: bank.Withdrawn, Amount: 5000, At: testTime}})
		decision := bank.Decide(bank.Close(testTime), state)
		if !decision.IsErr() {
			t.Fatal("closed an account with negative balance")
		}
		err := decision.UnwrapErr()
		if err.Kind != bank.BalanceIsNegative || err.Balance != -5000 {
			t.Fatalf("unexpected close error: %#v", err)
		}
	})
}

func TestClosedAccountRejectsEveryCommand(t *testing.T) {
	state := bank.Fold([]bank.Event{{Type: bank.Closed, At: testTime}})
	commands := []bank.Command{
		bank.Deposit(100, testTime),
		bank.Withdraw(100, testTime, option.Some(bank.Money(0))),
		bank.Close(testTime),
	}
	for _, command := range commands {
		decision := bank.Decide(command, state)
		if !decision.IsErr() || decision.UnwrapErr().Kind != bank.AlreadyClosed {
			t.Fatalf("command %v did not return AlreadyClosed", command.Type)
		}
	}
}

func TestStateEvolution(t *testing.T) {
	events := []bank.Event{
		{Type: bank.Deposited, Amount: 5000, At: testTime},
		{Type: bank.Withdrawn, Amount: 1000, At: testTime},
		{Type: bank.Withdrawn, Amount: 500, At: testTime},
		{Type: bank.Deposited, Amount: 10000, At: testTime},
	}
	if got := bank.Fold(events); got != (bank.State{Balance: 13500}) {
		t.Fatalf("got %#v, want balance 135.00 and open", got)
	}

	pairs := bank.EvolveWithStates(bank.Initial(), []bank.Event{
		{Type: bank.Deposited, Amount: 1000, At: testTime},
		{Type: bank.Closed, At: testTime},
	})
	if len(pairs) != 2 || pairs[0].State.Balance != 1000 || !pairs[1].State.Closed {
		t.Fatalf("unexpected post-event states: %#v", pairs)
	}
}

func TestNegativeAmountsRemainLiteralDomainOperations(t *testing.T) {
	deposit := bank.Decide(bank.Deposit(-1000, testTime), bank.Initial()).Unwrap()
	withdraw := bank.Decide(bank.Withdraw(-500, testTime, option.None[bank.Money]()), bank.Fold(deposit)).Unwrap()
	if got := bank.Fold(append(deposit, withdraw...)).Balance; got != -500 {
		t.Fatalf("got balance %s, want -5.00", got)
	}
}

func TestDecideRejectsBalanceOverflow(t *testing.T) {
	maximum, err := bank.ParseMoney("92233720368547758.07")
	if err != nil {
		t.Fatal(err)
	}
	state := bank.Fold([]bank.Event{{Type: bank.Deposited, Amount: maximum, At: testTime}})
	deposit := bank.Decide(bank.Deposit(1, testTime), state)
	if !deposit.IsErr() || deposit.UnwrapErr().Kind != bank.BalanceOverflow {
		t.Fatalf("unexpected deposit outcome: %#v", deposit)
	}

	withdraw := bank.Decide(bank.Withdraw(1, testTime, option.None[bank.Money]()), bank.State{Balance: bank.Money(-1 << 63)})
	if !withdraw.IsErr() || withdraw.UnwrapErr().Kind != bank.BalanceOverflow {
		t.Fatalf("unexpected withdrawal outcome: %#v", withdraw)
	}

	withdraw = bank.Decide(bank.Withdraw(bank.Money(-1<<63), testTime, option.None[bank.Money]()), bank.State{Balance: -1})
	if !withdraw.IsErr() || withdraw.UnwrapErr().Kind != bank.BalanceOverflow {
		t.Fatalf("unexpected minimum withdrawal outcome: %#v", withdraw)
	}
}

func TestMoneyRoundTrip(t *testing.T) {
	tests := map[string]bank.Money{
		"0":       0,
		"12.3":    1230,
		"+12.34":  1234,
		"-0.05":   -5,
		"-12.34":  -1234,
		" 99.99 ": 9999,
	}
	for input, want := range tests {
		got, err := bank.ParseMoney(input)
		if err != nil || got != want {
			t.Fatalf("ParseMoney(%q): got %v, %v; want %v", input, got, err, want)
		}
	}
	if bank.Money(-1234).String() != "-12.34" || bank.Money(5).String() != "0.05" {
		t.Fatal("unexpected Money.String output")
	}

	for _, input := range []string{"", ".50", "1.", "1.234", "one", "92233720368547759"} {
		if _, err := bank.ParseMoney(input); !errors.Is(err, bank.ErrInvalidMoney) {
			t.Fatalf("ParseMoney(%q) error = %v", input, err)
		}
	}
}
