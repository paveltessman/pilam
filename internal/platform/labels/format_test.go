package labels

import (
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

func TestDate(t *testing.T) {
	testData := map[string]struct {
		in   time.Time
		want string
	}{
		"display form": {date.Of(2027, time.February, 9), "09.02.2027"},
		"zero padded":  {date.Of(2027, time.December, 31), "31.12.2027"},
		"unset":        {time.Time{}, Empty},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := Date(tc.in); got != tc.want {
				t.Errorf("Date(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDelta(t *testing.T) {
	testData := map[string]struct {
		in   int
		want string
	}{
		"late":     {12, "+12д"},
		"early":    {-3, "-3д"},
		"on time":  {0, "0д"},
		"one late": {1, "+1д"},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := Delta(tc.in); got != tc.want {
				t.Errorf("Delta(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNumberGroupsThousands(t *testing.T) {
	testData := map[string]struct {
		in   int
		want string
	}{
		"units":        {7, "7"},
		"three digits": {594, "594"},
		"four digits":  {1234, "1" + nbsp + "234"},
		"exact groups": {1234567, "1" + nbsp + "234" + nbsp + "567"},
		"zero":         {0, "0"},
		"negative":     {-1500, "-1" + nbsp + "500"},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := Number(tc.in); got != tc.want {
				t.Errorf("Number(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDecimal(t *testing.T) {
	testData := map[string]struct {
		in   string
		want string
	}{
		"two places":      {"1234.50", "1" + nbsp + "234,50"},
		"one place":       {"1234.5", "1" + nbsp + "234,5"},
		"no fraction":     {"1234", "1" + nbsp + "234"},
		"trailing zeroes": {"100.00", "100,00"},
		"negative":        {"-42.75", "-42,75"},
		"below one":       {"0.99", "0,99"},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := Decimal(tc.in); got != tc.want {
				t.Errorf("Decimal(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDecimalPassesThroughWhatIsNotANumber(t *testing.T) {
	testData := map[string]string{
		"empty":            "",
		"already grouped":  "1 234,50",
		"trailing point":   "1234.",
		"leading point":    ".5",
		"two points":       "1.2.3",
		"words":            "n/a",
		"scientific":       "1e6",
		"sign only":        "-",
		"embedded spaces":  "1 234",
		"currency in text": "100 ₽",
	}
	for name, in := range testData {
		t.Run(name, func(t *testing.T) {
			if got := Decimal(in); got != in {
				t.Errorf("Decimal(%q) = %q, want it unchanged", in, got)
			}
		})
	}
}

func TestMoney(t *testing.T) {
	testData := map[string]struct {
		amount, currency string
		want             string
	}{
		"roubles":            {"1234.50", "RUB", "1" + nbsp + "234,50" + nbsp + "₽"},
		"yuan":               {"89.00", "CNY", "89,00" + nbsp + "¥"},
		"lowercase code":     {"89.00", "cny", "89,00" + nbsp + "¥"},
		"unknown code":       {"12.00", "ABC", "12,00" + nbsp + "ABC"},
		"no currency at all": {"12.00", "", "12,00"},
		"unset":              {"", "RUB", Empty},
		"whitespace":         {"   ", "RUB", Empty},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			got := Money(tc.amount, tc.currency)
			if got != tc.want {
				t.Errorf("Money(%q, %q) = %q, want %q", tc.amount, tc.currency, got, tc.want)
			}
		})
	}
}

func TestPlural(t *testing.T) {
	testData := []struct {
		n    int
		want string
	}{
		{0, "моделей"},
		{1, "модель"},
		{2, "модели"},
		{4, "модели"},
		{5, "моделей"},
		{10, "моделей"},
		{11, "моделей"}, // ends in 1, but is a teen
		{12, "моделей"},
		{14, "моделей"},
		{15, "моделей"},
		{21, "модель"},
		{22, "модели"},
		{25, "моделей"},
		{101, "модель"},
		{111, "моделей"}, // ends in 11, still a teen
		{112, "моделей"},
		{1002, "модели"},
		// Slack is signed, and a mean slack of -3 days is still "3 дня".
		{-1, "модель"},
		{-3, "модели"},
		{-11, "моделей"},
	}
	for _, tc := range testData {
		if got := Plural(tc.n, "модель", "модели", "моделей"); got != tc.want {
			t.Errorf("Plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestCountJoinsNumberAndNoun(t *testing.T) {
	if got, want := Count(5, "модель", "модели", "моделей"), "5"+nbsp+"моделей"; got != want {
		t.Errorf("Count = %q, want %q", got, want)
	}
	if got, want := Count(1234, "модель", "модели", "моделей"), "1"+nbsp+"234"+nbsp+"модели"; got != want {
		t.Errorf("Count = %q, want %q", got, want)
	}
}

func TestCountedNouns(t *testing.T) {
	testData := map[string]struct {
		got, want string
	}{
		"styles one":     {Styles(1), "1" + nbsp + "модель"},
		"styles many":    {Styles(60), "60" + nbsp + "моделей"},
		"colorways few":  {Colorways(3), "3" + nbsp + "цвета"},
		"colorways many": {Colorways(140), "140" + nbsp + "цветов"},
		"days one":       {Days(1), "1" + nbsp + "день"},
		"days few":       {Days(12), "12" + nbsp + "дней"},
		"days many":      {Days(22), "22" + nbsp + "дня"},
		"times one":      {Times(1), "1" + nbsp + "раз"},
		"times few":      {Times(3), "3" + nbsp + "раза"},
		"times many":     {Times(5), "5" + nbsp + "раз"},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

// Every separator this package inserts has to be non-breaking, or a card wraps
// "1" onto one line and "234 ₽" onto the next.
func TestSeparatorsNeverBreak(t *testing.T) {
	testData := map[string]string{
		"grouped number": Number(1234567),
		"money":          Money("1234.50", "RUB"),
		"count":          Count(1234, "модель", "модели", "моделей"),
	}
	for name, got := range testData {
		t.Run(name, func(t *testing.T) {
			if strings.ContainsRune(got, ' ') {
				t.Errorf("%q contains a breaking space", got)
			}
		})
	}
}
