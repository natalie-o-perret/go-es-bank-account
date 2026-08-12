package domain_test

import (
	"errors"
	"testing"

	bank "github.com/natalie-o-perret/go-es-bank-account/features/bank/domain"
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
