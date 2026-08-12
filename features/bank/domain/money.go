package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
