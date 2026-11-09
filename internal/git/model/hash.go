package model

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ShortHashLength is the number of hex digits shown for a short hash.
// JetBrains uses 8 (com.intellij.platform.vcs.VcsUtil.SHORT_HASH_LENGTH),
// which is one more than git's historical default of 7.
const ShortHashLength = 8

// SHA-1 and SHA-256 object names, matching GitObjectFormat.SHA1/SHA256.
const (
	sha1HexLen   = 40
	sha256HexLen = 64
)

// ErrInvalidHash is returned when a string is not a full object name:
// 40 hex digits (SHA-1) or 64 hex digits (SHA-256).
var ErrInvalidHash = errors.New("invalid git object hash")

// Hash is a full Git object name. The zero value is not a valid hash;
// use ParseHash.
//
// Equality is case-insensitive: git accepts both cases, and two names that
// differ only in case identify the same object. String() returns the hex
// in the case it was parsed with.
type Hash struct {
	hex string
}

// ParseHash validates a full object name. Abbreviated hashes and any other
// length are rejected. This is stricter than HashImpl.build, which accepts
// any hex length, and matches GitUtil.isPossibleFullHash.
func ParseHash(s string) (Hash, error) {
	if len(s) != sha1HexLen && len(s) != sha256HexLen {
		return Hash{}, fmt.Errorf("%w: length %d", ErrInvalidHash, len(s))
	}
	for _, c := range s {
		if !isHex(c) {
			return Hash{}, fmt.Errorf("%w: %q", ErrInvalidHash, s)
		}
	}
	return Hash{hex: s}, nil
}

// MustParseHash is ParseHash for known-valid literals. It panics otherwise.
func MustParseHash(s string) Hash {
	h, err := ParseHash(s)
	if err != nil {
		panic(err)
	}
	return h
}

// String returns the full hex object name, or "" for the zero Hash.
func (h Hash) String() string { return h.hex }

// Short returns the first ShortHashLength hex digits.
// A hash shorter than that (only the zero value) is returned unchanged.
func (h Hash) Short() string {
	if len(h.hex) <= ShortHashLength {
		return h.hex
	}
	return h.hex[:ShortHashLength]
}

// IsZero reports whether h is the zero Hash.
func (h Hash) IsZero() bool { return h.hex == "" }

// Equal reports whether two hashes name the same object.
func (h Hash) Equal(other Hash) bool {
	return strings.EqualFold(h.hex, other.hex)
}

// IsFullHash reports whether s is a valid full object name.
func IsFullHash(s string) bool {
	_, err := ParseHash(s)
	return err == nil
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// IsHexString reports whether s is non-empty and contains only hex digits.
// GitUtil.isPossibleHash also requires length 7..64; this helper does not,
// so callers can apply their own length rule.
func IsHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !isHex(c) {
			return false
		}
	}
	return true
}

// IsPossibleHash matches GitUtil.isPossibleHash: 7 to 64 hex digits.
func IsPossibleHash(s string) bool {
	n := len(s)
	if n < 7 || n > 64 {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII || !isHex(r) {
			return false
		}
	}
	return true
}
