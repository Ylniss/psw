package passgen

import (
	"strings"
	"testing"
)

func TestDefaultOptionsAreValid(t *testing.T) {
	if err := DefaultOptions().Validate(); err != nil {
		t.Fatalf("DefaultOptions invalid: %v", err)
	}
}

func TestGenerateRespectsLength(t *testing.T) {
	for _, n := range []int{1, 8, 16, 32, 64} {
		o := DefaultOptions()
		o.Length = n
		o.MinDigits, o.MinSymbols, o.MinUppercase, o.MinLowercase = 0, 0, 0, 0
		got, err := Generate(o)
		if err != nil {
			t.Fatalf("len=%d: %v", n, err)
		}
		if len(got) != n {
			t.Fatalf("len=%d: got %d", n, len(got))
		}
	}
}

func TestGenerateRespectsMinima(t *testing.T) {
	o := Options{Length: 20, MinDigits: 4, MinSymbols: 6, MinUppercase: 2, MinLowercase: 2, AllowRepeat: true}
	for i := range 50 {
		got, err := Generate(o)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		d := countAny(got, DigitPool)
		s := countAny(got, SymbolPool)
		u := countAny(got, UpperPool)
		l := countAny(got, LowerPool)
		if d < o.MinDigits || s < o.MinSymbols || u < o.MinUppercase || l < o.MinLowercase {
			t.Fatalf("iter %d: %q d=%d s=%d u=%d l=%d", i, got, d, s, u, l)
		}
		if d+s+u+l != len(got) {
			t.Fatalf("iter %d: %q has chars outside the four pools", i, got)
		}
	}
}

func TestGenerateAllowRepeatFalseUnique(t *testing.T) {
	o := Options{Length: 30, MinDigits: 4, MinSymbols: 6, MinUppercase: 5, MinLowercase: 5, AllowRepeat: false}
	for i := range 50 {
		got, err := Generate(o)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		seen := make(map[byte]bool, len(got))
		for j, c := range []byte(got) {
			if seen[c] {
				t.Fatalf("iter %d: %q has repeat at index %d", i, got, j)
			}
			seen[c] = true
		}
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want string
	}{
		{"length below one", Options{Length: 0}, "length"},
		{"min sum exceeds length", Options{Length: 10, MinDigits: 5, MinSymbols: 6, AllowRepeat: true}, "minimums sum"},
		{"no repeat exceeds pool", Options{Length: 30, MinSymbols: 25, AllowRepeat: false}, "min_symbols"},
		{"negative min", Options{Length: 16, MinDigits: -1, AllowRepeat: true}, "min_digits"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func countAny(s, pool string) int {
	n := 0
	for _, c := range []byte(s) {
		if strings.IndexByte(pool, c) >= 0 {
			n++
		}
	}
	return n
}
