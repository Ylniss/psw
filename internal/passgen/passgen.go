// Package passgen generates random passwords with per-category minimums.
// Minima are floors, not exact counts: "at least 4 digits" leaves the rest
// of the length free to fill from the full pool.
package passgen

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

const (
	DigitPool  = "0123456789"
	SymbolPool = "!@#$%^&*()-_=+[]{}<>?,./"
	UpperPool  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	LowerPool  = "abcdefghijklmnopqrstuvwxyz"
)

type Options struct {
	Length       int
	MinDigits    int
	MinSymbols   int
	MinUppercase int
	MinLowercase int
	AllowRepeat  bool
}

func DefaultOptions() Options {
	return Options{
		Length:       16,
		MinDigits:    4,
		MinSymbols:   6,
		MinUppercase: 1,
		MinLowercase: 1,
		AllowRepeat:  true,
	}
}

func (o Options) Validate() error {
	if o.Length < 1 {
		return fmt.Errorf("length=%d, must be >= 1", o.Length)
	}
	if o.MinDigits < 0 {
		return fmt.Errorf("min_digits=%d, must be >= 0", o.MinDigits)
	}
	if o.MinSymbols < 0 {
		return fmt.Errorf("min_symbols=%d, must be >= 0", o.MinSymbols)
	}
	if o.MinUppercase < 0 {
		return fmt.Errorf("min_uppercase=%d, must be >= 0", o.MinUppercase)
	}
	if o.MinLowercase < 0 {
		return fmt.Errorf("min_lowercase=%d, must be >= 0", o.MinLowercase)
	}
	sum := o.MinDigits + o.MinSymbols + o.MinUppercase + o.MinLowercase
	if sum > o.Length {
		return fmt.Errorf("length=%d but minimums sum to %d (digits=%d + symbols=%d + uppercase=%d + lowercase=%d)",
			o.Length, sum, o.MinDigits, o.MinSymbols, o.MinUppercase, o.MinLowercase)
	}
	if !o.AllowRepeat {
		if o.MinDigits > len(DigitPool) {
			return fmt.Errorf("min_digits=%d exceeds digit pool size %d with allow_repeat=false", o.MinDigits, len(DigitPool))
		}
		if o.MinSymbols > len(SymbolPool) {
			return fmt.Errorf("min_symbols=%d exceeds symbol pool size %d with allow_repeat=false", o.MinSymbols, len(SymbolPool))
		}
		if o.MinUppercase > len(UpperPool) {
			return fmt.Errorf("min_uppercase=%d exceeds uppercase pool size %d with allow_repeat=false", o.MinUppercase, len(UpperPool))
		}
		if o.MinLowercase > len(LowerPool) {
			return fmt.Errorf("min_lowercase=%d exceeds lowercase pool size %d with allow_repeat=false", o.MinLowercase, len(LowerPool))
		}
		fullSize := len(DigitPool) + len(SymbolPool) + len(UpperPool) + len(LowerPool)
		if o.Length > fullSize {
			return fmt.Errorf("length=%d exceeds total pool size %d with allow_repeat=false", o.Length, fullSize)
		}
	}
	return nil
}

func Generate(o Options) (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	categories := []struct {
		pool string
		min  int
	}{
		{DigitPool, o.MinDigits},
		{SymbolPool, o.MinSymbols},
		{UpperPool, o.MinUppercase},
		{LowerPool, o.MinLowercase},
	}
	pick := pickWithRepeat
	if !o.AllowRepeat {
		used := make(map[byte]bool, o.Length)
		pick = func(buf *[]byte, pool string, n int) error {
			return pickDistinct(buf, used, pool, n)
		}
	}
	buf := make([]byte, 0, o.Length)
	for _, c := range categories {
		if err := pick(&buf, c.pool, c.min); err != nil {
			return "", err
		}
	}
	full := DigitPool + SymbolPool + UpperPool + LowerPool
	if err := pick(&buf, full, o.Length-len(buf)); err != nil {
		return "", err
	}
	if err := shuffle(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// pickWithRepeat picks n random bytes from pool (with replacement) and
// appends them.
func pickWithRepeat(buf *[]byte, pool string, n int) error {
	for range n {
		idx, err := randIndex(len(pool))
		if err != nil {
			return err
		}
		*buf = append(*buf, pool[idx])
	}
	return nil
}

// pickDistinct appends n random bytes from pool that aren't already in `used`.
// Bytes are added to both buf and used. Partial Fisher-Yates over the unused
// candidates avoids shuffling the whole pool when n is small.
func pickDistinct(buf *[]byte, used map[byte]bool, pool string, n int) error {
	if n == 0 {
		return nil
	}
	avail := make([]byte, 0, len(pool))
	for i := range len(pool) {
		if !used[pool[i]] {
			avail = append(avail, pool[i])
		}
	}
	if n > len(avail) {
		return errors.New("internal: pool too small for distinct picks (validation should have caught this)")
	}
	for i := range n {
		j, err := randIndex(len(avail) - i)
		if err != nil {
			return err
		}
		j += i
		avail[i], avail[j] = avail[j], avail[i]
		used[avail[i]] = true
		*buf = append(*buf, avail[i])
	}
	return nil
}

// shuffle reorders buf in place with Fisher-Yates over crypto/rand.
func shuffle(buf []byte) error {
	for i := len(buf) - 1; i > 0; i-- {
		j, err := randIndex(i + 1)
		if err != nil {
			return err
		}
		buf[i], buf[j] = buf[j], buf[i]
	}
	return nil
}

func randIndex(n int) (int, error) {
	bn, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(bn.Int64()), nil
}
