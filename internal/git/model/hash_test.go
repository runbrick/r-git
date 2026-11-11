package model

import (
	"errors"
	"strings"
	"testing"
)

func TestParseHash(t *testing.T) {
	sha1 := strings.Repeat("a", 40)
	sha256 := strings.Repeat("b", 64)
	upper := strings.Repeat("A", 40)

	tests := []struct {
		name    string
		in      string
		wantErr bool
		short   string
	}{
		{name: "sha1", in: sha1, short: sha1[:ShortHashLength]},
		{name: "sha256", in: sha256, short: sha256[:ShortHashLength]},
		{name: "upper hex", in: upper, short: upper[:ShortHashLength]},
		{name: "mixed", in: "0123456789abcdef0123456789ABCDEF01234567", short: "01234567"},
		{name: "empty", in: "", wantErr: true},
		{name: "too short", in: "abc", wantErr: true},
		{name: "abbreviated 7", in: "abcdef0", wantErr: true},
		{name: "39", in: strings.Repeat("a", 39), wantErr: true},
		{name: "41", in: strings.Repeat("a", 41), wantErr: true},
		{name: "63", in: strings.Repeat("a", 63), wantErr: true},
		{name: "non hex", in: strings.Repeat("g", 40), wantErr: true},
		{name: "space", in: strings.Repeat("a", 39) + " ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := ParseHash(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidHash) {
					t.Fatalf("ParseHash(%q) err = %v, want ErrInvalidHash", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHash(%q) unexpected err %v", tt.in, err)
			}
			if h.String() != tt.in {
				t.Fatalf("String() = %q, want %q", h.String(), tt.in)
			}
			if h.Short() != tt.short {
				t.Fatalf("Short() = %q, want %q", h.Short(), tt.short)
			}
		})
	}
}

func TestHashEqualFold(t *testing.T) {
	a := MustParseHash(strings.Repeat("ab", 20))
	b := MustParseHash(strings.ToUpper(strings.Repeat("ab", 20)))
	c := MustParseHash(strings.Repeat("cd", 20))
	if !a.Equal(b) {
		t.Fatal("hashes that differ only in case must be equal")
	}
	if a.Equal(c) {
		t.Fatal("different hashes compared equal")
	}
	if !a.Equal(a) {
		t.Fatal("hash must equal itself")
	}
	var zero Hash
	if !zero.IsZero() || zero.Short() != "" || a.Equal(zero) {
		t.Fatalf("zero hash = %+v short %q", zero, zero.Short())
	}
}

func TestIsPossibleHash(t *testing.T) {
	tests := []struct {
		in   string
		full bool
		poss bool
	}{
		{strings.Repeat("a", 40), true, true},
		{strings.Repeat("a", 64), true, true},
		{"abcdef0", false, true},
		{strings.Repeat("a", 6), false, false},
		{strings.Repeat("a", 65), false, false},
		{"", false, false},
		{"zzzzzzz", false, false},
	}
	for _, tt := range tests {
		if got := IsFullHash(tt.in); got != tt.full {
			t.Errorf("IsFullHash(%q) = %v, want %v", tt.in, got, tt.full)
		}
		if got := IsPossibleHash(tt.in); got != tt.poss {
			t.Errorf("IsPossibleHash(%q) = %v, want %v", tt.in, got, tt.poss)
		}
	}
}
