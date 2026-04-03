package random

import (
	"testing"
)

func TestSeqLength(t *testing.T) {
	lengths := []int{0, 1, 10, 32, 64, 128}
	for _, n := range lengths {
		s := Seq(n)
		if len([]rune(s)) != n {
			t.Errorf("Seq(%d) returned string of length %d", n, len([]rune(s)))
		}
	}
}

func TestSeqCharset(t *testing.T) {
	allChars := make(map[rune]bool)
	for _, r := range allSeq {
		allChars[r] = true
	}

	s := Seq(1000)
	for i, r := range s {
		if !allChars[r] {
			t.Errorf("Seq produced invalid character %q at index %d", r, i)
		}
	}
}

func TestSeqUniqueness(t *testing.T) {
	// Generate several strings and check they're not all identical
	seen := make(map[string]bool)
	for range 10 {
		seen[Seq(32)] = true
	}
	if len(seen) < 2 {
		t.Error("Seq(32) produced identical strings across 10 calls")
	}
}

func TestSeqEmpty(t *testing.T) {
	s := Seq(0)
	if s != "" {
		t.Errorf("Seq(0) should return empty string, got %q", s)
	}
}

func TestNumRange(t *testing.T) {
	for _, n := range []int{1, 5, 10, 100, 1000} {
		for range 100 {
			r := Num(n)
			if r < 0 || r >= n {
				t.Errorf("Num(%d) returned %d, expected [0, %d)", n, r, n)
			}
		}
	}
}

func TestNumOne(t *testing.T) {
	for range 50 {
		r := Num(1)
		if r != 0 {
			t.Errorf("Num(1) should always return 0, got %d", r)
		}
	}
}
