package model

// NaturalCompare orders strings the way IntelliJ's NaturalComparator does
// with its default arguments (ignoreCase = true, likeFileNames = false).
//
// GitReference.REFS_NAMES_COMPARATOR is that comparator, and branch and
// tag ordering goes through it. The rules, from NaturalComparator:
//
//   - A run of spaces and digits is a number. Leading spaces and zeros are
//     skipped before the numeric value is compared.
//   - The number with more significant digits is greater.
//   - Equal-length numbers compare by their digits.
//   - If the values are equal, the number with more characters including
//     leading spaces and zeros is greater.
//   - If that is also equal, the leading spaces and zeros compare by code point.
//   - Other characters compare case-insensitively. When the case-insensitive
//     comparison ties, a case-sensitive comparison breaks the tie.
//   - A space sorts after characters that sit between space and '0'
//     ('!', '"', '#', ...), which keeps the order transitive.
//
// A nil string in Java sorts first. Go has no nil strings; empty is just
// empty, and it sorts before any non-empty string by the length rule.
func NaturalCompare(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	return naturalCompare(s1, s2, true)
}

func naturalCompare(s1, s2 string, ignoreCase bool) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	i, j := 0, 0
	n1, n2 := len(r1), len(r2)
	for i < n1 && j < n2 {
		ch1 := r1[i]
		ch2 := r2[j]
		if isDigitOrSpace(ch1) && isDigitOrSpace(ch2) {
			start1 := skipRune(r1, skipRune(r1, i, ' '), '0')
			start2 := skipRune(r2, skipRune(r2, j, ' '), '0')
			end1 := skipDigits(r1, start1)
			end2 := skipDigits(r2, start2)

			lengthDiff := (end1 - start1) - (end2 - start2)
			if lengthDiff != 0 {
				return clamp(lengthDiff)
			}
			if diff := compareRuneRange(r1, r2, start1, start2, end1); diff != 0 {
				return diff
			}
			fullLengthDiff := (end1 - i) - (end2 - j)
			if fullLengthDiff != 0 {
				return clamp(fullLengthDiff)
			}
			if diff := compareRuneRange(r1, r2, i, j, start1); diff != 0 {
				return diff
			}
			i = end1
			j = end2
			continue
		}
		if diff := compareRunes(ch1, ch2, ignoreCase); diff != 0 {
			return diff
		}
		i++
		j++
	}
	if i < n1 {
		return 1
	}
	if j < n2 {
		return -1
	}
	if n1 != n2 {
		return clamp(n1 - n2)
	}
	if ignoreCase {
		return naturalCompare(s1, s2, false)
	}
	return 0
}

func isDigitOrSpace(c rune) bool {
	return c == ' ' || (c >= '0' && c <= '9')
}

func skipRune(s []rune, start int, c rune) int {
	for start < len(s) && s[start] == c {
		start++
	}
	return start
}

func skipDigits(s []rune, start int) int {
	for start < len(s) && s[start] >= '0' && s[start] <= '9' {
		start++
	}
	return start
}

func compareRuneRange(s1, s2 []rune, offset1, offset2, end1 int) int {
	for i, j := offset1, offset2; i < end1; i, j = i+1, j+1 {
		diff := int(s1[i]) - int(s2[j])
		if diff != 0 {
			return clamp(diff)
		}
	}
	return 0
}

func compareRunes(ch1, ch2 rune, ignoreCase bool) int {
	// Transitivity fix from NaturalComparator.compareChars: a space sorts
	// after characters strictly between ' ' and '0'.
	if ch1 == ' ' && ch2 > ' ' && ch2 < '0' {
		return 1
	}
	if ch2 == ' ' && ch1 > ' ' && ch1 < '0' {
		return -1
	}
	d := int(ch1) - int(ch2)
	if d == 0 || !ignoreCase {
		return clamp(d)
	}
	// ASCII case folding matches StringUtilRt for the letters branch
	// names are made of. Non-ASCII letters compare by code point, which
	// is the same result Java produces when toUpperCase leaves them
	// unchanged (the common case for letters outside the ASCII range
	// that the comparator sees in ref names).
	u1 := foldUpper(ch1)
	u2 := foldUpper(ch2)
	d = int(u1) - int(u2)
	if d != 0 {
		d = int(foldLower(u1)) - int(foldLower(u2))
	}
	return clamp(d)
}

func foldUpper(c rune) rune {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

func foldLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// clamp keeps the sign and a small magnitude so a difference of code
// points never overflows an int conversion that a caller treats as a
// boolean. Java returns the raw char subtraction, which fits in an int.
func clamp(d int) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	default:
		return 0
	}
}
