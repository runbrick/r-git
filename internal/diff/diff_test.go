package diff

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// Comparable cases ported from platform/diff-impl/tests/testSrc/com/intellij/
// diff/comparison/LineComparisonUtilTest.kt (underscores denote newlines there).
func TestIntelliJLineCases(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want []Change
	}{
		{"", "", nil}, {"x", "x", nil}, {"x\ny\nz\n", "x\ny\nz\n", nil},
		{"x\n", "y\n", []Change{{0, 1, 0, 1}}},
		{"x", "", []Change{{0, 1, 0, 1}}}, {"", "x", []Change{{0, 1, 0, 1}}},
		{"z\nx", "z\ny", []Change{{1, 2, 1, 2}}},
		{"x", "x\n", []Change{{1, 1, 1, 2}}}, {"x\n", "x", []Change{{1, 2, 1, 1}}},
		{"x\ny", "n\nm", []Change{{0, 2, 0, 2}}},
		{"x\ny\nz", "n\ny\nm", []Change{{0, 1, 0, 1}, {2, 3, 2, 3}}},
		{"x\ny\nz", "n\nk\ny", []Change{{0, 1, 0, 2}, {2, 3, 3, 3}}},
		{"x\ny\nz", "y", []Change{{0, 1, 0, 0}, {2, 3, 1, 1}}},
		{"a\nb\nx", "x\nm\nn", []Change{{0, 2, 0, 0}, {3, 3, 1, 3}}},
		{"x\n", "", []Change{{0, 1, 0, 0}}}, {"", "x\n", []Change{{0, 0, 0, 1}}},
	} {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			for _, p := range []Policy{Default, IgnoreWhitespace, IgnoreTrailingWhitespace} {
				if got := Compare(tt.a, tt.b, p).Changes; !reflect.DeepEqual(got, tt.want) {
					t.Fatalf("policy %d got %v want %v", p, got, tt.want)
				}
			}
		})
	}
}

func TestPolicies(t *testing.T) {
	for _, tt := range []struct {
		a, b  string
		p     Policy
		equal bool
	}{
		{"x ", " x", Default, false}, {"x \t", "\tx", IgnoreWhitespace, true},
		{"x \t", "x", IgnoreTrailingWhitespace, true}, {" x", "x", IgnoreTrailingWhitespace, false},
		{"x y", "xy", IgnoreWhitespace, true}, {"x\ny", "xy", IgnoreWhitespace, false},
		{"世界\u2003", "世界", IgnoreTrailingWhitespace, true},
	} {
		if got := len(Compare(tt.a, tt.b, tt.p).Changes) == 0; got != tt.equal {
			t.Fatalf("%q/%q policy %d equal %v", tt.a, tt.b, tt.p, got)
		}
	}
}

// WordComparisonUtilTest.testSimpleCases and testPunctuation, exact comparable spans.
func TestIntelliJWordCases(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want Words
	}{
		{"x z", "y z", Words{[]Span{{0, 1}}, []Span{{0, 1}}}},
		{"x,y", "x", Words{[]Span{{1, 3}}, nil}},
		{"x,y", "y", Words{[]Span{{0, 2}}, nil}},
		{".x=", ".!=", Words{[]Span{{1, 2}}, []Span{{1, 2}}}},
		{"X xyz1 Z", "X xyz2 Z", Words{[]Span{{2, 6}}, []Span{{2, 6}}}},
		{"x ... z", "x ... y", Words{[]Span{{6, 7}}, []Span{{6, 7}}}},
		{"你好 世界", "你好 地球", Words{[]Span{{7, 13}}, []Span{{7, 13}}}},
	} {
		if got := WordDiff(tt.a, tt.b, Default); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%q/%q got %#v want %#v", tt.a, tt.b, got, tt.want)
		}
	}
	if got := WordDiff("x \t", "x", IgnoreTrailingWhitespace); len(got.A)+len(got.B) != 0 {
		t.Fatal(got)
	}
}

// Validate both reconstruction and minimality against a small independent LCS oracle.
func TestMyersRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 3000; trial++ {
		a, b := make([]string, rng.Intn(20)), make([]string, rng.Intn(20))
		for i := range a {
			a[i] = string(rune('a' + rng.Intn(5)))
		}
		for i := range b {
			b[i] = string(rune('a' + rng.Intn(5)))
		}
		changes := Lines(a, b, Default)
		var rebuilt []string
		at, cost := 0, 0
		for _, c := range changes {
			rebuilt = append(rebuilt, a[at:c.StartA]...)
			rebuilt = append(rebuilt, b[c.StartB:c.EndB]...)
			at = c.EndA
			cost += c.EndA - c.StartA + c.EndB - c.StartB
		}
		rebuilt = append(rebuilt, a[at:]...)
		if strings.Join(rebuilt, "|") != strings.Join(b, "|") {
			t.Fatalf("reconstruct %v %v %v", a, b, changes)
		}
		dp := make([]int, len(b)+1)
		for _, x := range a {
			prev := 0
			for j, y := range b {
				old := dp[j+1]
				if x == y {
					dp[j+1] = prev + 1
				} else {
					dp[j+1] = max(dp[j+1], dp[j])
				}
				prev = old
			}
		}
		if want := len(a) + len(b) - 2*dp[len(b)]; cost != want {
			t.Fatalf("nonminimal %v %v: %v cost %d want %d", a, b, changes, cost, want)
		}
	}
}
