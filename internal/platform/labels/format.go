package labels

import (
	"strconv"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

const (
	Empty      = "—"
	decimalSep = ","

	// The thousands separator and the space before a unit.
	nbsp = "\u00a0"
)

var currencySymbols = map[string]string{
	"RUB": "₽",
	"CNY": "¥",
}

func Date(d time.Time) string {
	if d.IsZero() {
		return Empty
	}
	return date.Format(d)
}

// Delta renders a signed count of days: "+12д", "-3д", "0д".
func Delta(days int) string {
	s := strconv.Itoa(days) + "д"
	if days > 0 {
		return "+" + s
	}
	return s
}

func Number(n int) string { return Decimal(strconv.Itoa(n)) }

// Decimal renders a decimal number from the format of SQL and pgx ("1234.50").
//
// Anything that is not a decimal number comes back unchanged.
func Decimal(s string) string {
	sign, digits := "", s
	if rest, found := strings.CutPrefix(digits, "-"); found {
		sign, digits = "-", rest
	}

	whole, frac, hasFrac := strings.Cut(digits, ".")
	if !isDigits(whole) || (hasFrac && !isDigits(frac)) {
		return s
	}

	out := sign + group(whole)
	if hasFrac {
		out += decimalSep + frac
	}
	return out
}

// Money renders an amount and its currency: Money("1234.50", "RUB") is
// "1 234,50 ₽".
// An amount that is not there reads as Empty rather than as zero.
func Money(amount, currency string) string {
	if strings.TrimSpace(amount) == "" {
		return Empty
	}

	unit := currency
	if symbol, known := currencySymbols[strings.ToUpper(currency)]; known {
		unit = symbol
	}
	if unit == "" {
		return Decimal(amount)
	}
	return Decimal(amount) + nbsp + unit
}

// Plural picks the form of a noun that agrees with n.
//
// one (1 модель), few (2 модели), many (5 моделей) — with a correction for the teens,
// where 11 through 14 all take the many form.
func Plural(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}
	switch {
	case n%100 >= 11 && n%100 <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	default:
		return many
	}
}

// Count is a number and the noun it counts, agreeing:
// Count(5, "модель", "модели", "моделей") is "5 моделей".
func Count(n int, one, few, many string) string {
	return Number(n) + nbsp + Plural(n, one, few, many)
}

// Chars counts the characters of a length rule, in a message and in the hint
// that states the rule before the user meets it.
func Chars(n int) string { return Count(n, "символ", "символа", "символов") }

// The nouns the screens count. Styles is the drop heading on S2, Days is S5's
// slip summary and S7's mean slack, Times is how often a forecast has moved.
func Styles(n int) string    { return Count(n, "модель", "модели", "моделей") }
func Colorways(n int) string { return Count(n, "цвет", "цвета", "цветов") }
func Days(n int) string      { return Count(n, "день", "дня", "дней") }
func Times(n int) string     { return Count(n, "раз", "раза", "раз") }

// group inserts the separator every three digits from the right.
func group(digits string) string {
	var b strings.Builder
	for i := range len(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(nbsp)
		}
		b.WriteByte(digits[i])
	}
	return b.String()
}

// isDigits reports whether s is one or more ASCII digits and nothing else. The
// empty string is not a number, which is what rejects "1234." and ".5".
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
