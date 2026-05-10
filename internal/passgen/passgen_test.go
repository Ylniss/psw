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
	for i := 0; i < 50; i++ {
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
	for i := 0; i < 50; i++ {
		got, err := Generate(o)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		seen := make(map[byte]bool, len(got))
		for j := 0; j < len(got); j++ {
			if seen[got[j]] {
				t.Fatalf("iter %d: %q has repeat at index %d", i, got, j)
			}
			seen[got[j]] = true
		}
	}
}

func TestValidateLengthBelowOne(t *testing.T) {
	err := Options{Length: 0}.Validate()
	if err == nil || !strings.Contains(err.Error(), "length") {
		t.Fatalf("want length error, got %v", err)
	}
}

func TestValidateMinSumExceedsLength(t *testing.T) {
	o := Options{Length: 10, MinDigits: 5, MinSymbols: 6, AllowRepeat: true}
	err := o.Validate()
	if err == nil || !strings.Contains(err.Error(), "minimums sum") {
		t.Fatalf("want sum error, got %v", err)
	}
}

func TestValidateNoRepeatExceedsPool(t *testing.T) {
	o := Options{Length: 30, MinSymbols: 25, AllowRepeat: false}
	err := o.Validate()
	if err == nil || !strings.Contains(err.Error(), "min_symbols") {
		t.Fatalf("want symbol-pool error, got %v", err)
	}
}

func TestValidateNegativeMin(t *testing.T) {
	o := Options{Length: 16, MinDigits: -1, AllowRepeat: true}
	err := o.Validate()
	if err == nil || !strings.Contains(err.Error(), "min_digits") {
		t.Fatalf("want negative min_digits error, got %v", err)
	}
}

func countAny(s, pool string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(pool, s[i]) >= 0 {
			n++
		}
	}
	return n
}
