package domain_test

import (
	"errors"
	"testing"

	bank "github.com/natalie-o-perret/go-es-bank-account/features/bank/domain"
	"github.com/natalie-o-perret/go-functionalish/kv"
	"github.com/natalie-o-perret/go-functionalish/seq"
	"github.com/natalie-o-perret/go-functionalish/slice"
)

func TestMoneyRoundTrip(t *testing.T) {
	tests := map[string]bank.Money{
		"0":       0,
		"12.3":    1230,
		"+12.34":  1234,
		"-0.05":   -5,
		"-12.34":  -1234,
		" 99.99 ": 9999,
	}
	kv.Of(tests).ToSeq().Iter(func(test seq.Pair[string, bank.Money]) {
		got, err := bank.ParseMoney(test.First)
		if err != nil || got != test.Second {
			t.Fatalf("ParseMoney(%q): got %v, %v; want %v", test.First, got, err, test.Second)
		}
	})
	if bank.Money(-1234).String() != "-12.34" || bank.Money(5).String() != "0.05" {
		t.Fatal("unexpected Money.String output")
	}

	slice.Of("", ".50", "1.", "1.234", "one", "92233720368547759").Iter(func(input string) {
		if _, err := bank.ParseMoney(input); !errors.Is(err, bank.ErrInvalidMoney) {
			t.Fatalf("ParseMoney(%q) error = %v", input, err)
		}
	})
}
