// Package diff compares text without depending on Git or the UI.
package diff

import (
	"strings"
	"unicode"
)

type Policy uint8

const (
	Default Policy = iota
	IgnoreWhitespace
	IgnoreTrailingWhitespace
)

// Change is a pair of half-open ranges. An empty range is an insertion/deletion.
type Change struct{ StartA, EndA, StartB, EndB int }

// Span uses UTF-8 byte offsets, suitable for slicing the original string.
type Span struct{ Start, End int }

type Words struct{ A, B []Span }

type Result struct {
	A, B    []string
	Changes []Change
	// Inner contains one entry per paired line of each replacement block.
	Inner map[[2]int]Words
}

func normalize(s string, policy Policy) string {
	switch policy {
	case IgnoreWhitespace:
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, s)
	case IgnoreTrailingWhitespace:
		return strings.TrimRightFunc(s, unicode.IsSpace)
	default:
		return s
	}
}

// Compare preserves a terminal empty line, matching IntelliJ's line offsets.
func Compare(a, b string, policy Policy) Result {
	r := Result{A: strings.Split(a, "\n"), B: strings.Split(b, "\n"), Inner: make(map[[2]int]Words)}
	r.Changes = Lines(r.A, r.B, policy)
	for _, c := range r.Changes {
		for i := 0; i < min(c.EndA-c.StartA, c.EndB-c.StartB); i++ {
			x, y := c.StartA+i, c.StartB+i
			r.Inner[[2]int{x, y}] = WordDiff(r.A[x], r.B[y], policy)
		}
	}
	return r
}

// Lines uses Myers' shortest edit path, preferring deletion on ties. Common
// prefixes/suffixes are stripped first. The trace has a fixed memory ceiling;
// pathological inputs exceeding it become one replacement (see README.md).
func Lines(a, b []string, policy Policy) []Change {
	x, y := make([]string, len(a)), make([]string, len(b))
	for i := range a {
		x[i] = normalize(a[i], policy)
	}
	for i := range b {
		y[i] = normalize(b[i], policy)
	}
	return sequence(x, y)
}

func sequence(a, b []string) []Change {
	prefix := 0
	for prefix < min(len(a), len(b)) && a[prefix] == b[prefix] {
		prefix++
	}
	endA, endB := len(a), len(b)
	for endA > prefix && endB > prefix && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	if prefix == endA && prefix == endB {
		return nil
	}
	x, y := a[prefix:endA], b[prefix:endB]
	if len(x) == 0 || len(y) == 0 {
		return []Change{{prefix, endA, prefix, endB}}
	}
	limit := len(x) + len(y)
	v := make([]int, 2*limit+3)
	offset := limit + 1
	var trace [][]int
	cells := 0
	for d := 0; d <= limit; d++ {
		cells += 2*d + 1
		if cells > 8*1024*1024 {
			return []Change{{prefix, endA, prefix, endB}}
		}
		layer := make([]int, 2*d+1)
		for k := -d; k <= d; k += 2 {
			var i int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				i = v[offset+k+1]
			} else {
				i = v[offset+k-1] + 1
			}
			j := i - k
			for i < len(x) && j < len(y) && x[i] == y[j] {
				i++
				j++
			}
			v[offset+k] = i
			layer[k+d] = i
			if i == len(x) && j == len(y) {
				trace = append(trace, layer)
				return backtrack(trace, len(x), len(y), prefix)
			}
		}
		trace = append(trace, layer)
	}
	return nil
}

func backtrack(trace [][]int, x, y, offset int) []Change {
	endX, endY := x, y
	// Collect matching pairs backwards; each gap between matches is one change.
	type pair struct{ a, b int }
	var matches []pair
	for d := len(trace) - 1; d > 0; d-- {
		k := x - y
		prev := trace[d-1]
		get := func(k int) int {
			if k < -(d-1) || k > d-1 {
				return 0
			}
			return prev[k+d-1]
		}
		pk := k - 1
		if k == -d || (k != d && get(k-1) < get(k+1)) {
			pk = k + 1
		}
		px := get(pk)
		py := px - pk
		for x > px && y > py {
			x--
			y--
			matches = append(matches, pair{x, y})
		}
		x, y = px, py
	}
	for x > 0 && y > 0 {
		x--
		y--
		matches = append(matches, pair{x, y})
	}
	var changes []Change
	a, b := 0, 0
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if a < m.a || b < m.b {
			changes = append(changes, Change{a + offset, m.a + offset, b + offset, m.b + offset})
		}
		a, b = m.a+1, m.b+1
	}
	if a < endX || b < endY {
		changes = append(changes, Change{a + offset, endX + offset, b + offset, endY + offset})
	}
	return changes
}

type token struct {
	value      string
	start, end int
}

func tokens(s string, policy Policy) []token {
	var result []token
	start, kind := -1, 0
	flush := func(end int) {
		if start >= 0 {
			value := s[start:end]
			if normalize(value, policy) != "" {
				result = append(result, token{value, start, end})
			}
			start = -1
		}
	}
	for i, r := range s {
		next := 3
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			next = 1
		} else if unicode.IsSpace(r) {
			next = 2
		}
		if next != kind || next == 3 {
			flush(i)
			start = i
			kind = next
		} else if start < 0 {
			start = i
		}
	}
	flush(len(s))
	return result
}

func WordDiff(a, b string, policy Policy) Words {
	if normalize(a, policy) == normalize(b, policy) {
		return Words{}
	}
	if policy == IgnoreTrailingWhitespace {
		a = strings.TrimRightFunc(a, unicode.IsSpace)
		b = strings.TrimRightFunc(b, unicode.IsSpace)
	}
	x, y := tokens(a, policy), tokens(b, policy)
	xv, yv := make([]string, len(x)), make([]string, len(y))
	for i := range x {
		xv[i] = x[i].value
	}
	for i := range y {
		yv[i] = y[i].value
	}
	var result Words
	for _, c := range sequence(xv, yv) {
		if c.StartA < c.EndA {
			result.A = append(result.A, Span{x[c.StartA].start, x[c.EndA-1].end})
		}
		if c.StartB < c.EndB {
			result.B = append(result.B, Span{y[c.StartB].start, y[c.EndB-1].end})
		}
	}
	return result
}
