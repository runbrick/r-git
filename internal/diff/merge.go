package diff

import (
	"slices"
	"strings"
)

type Side uint8

const (
	Unresolved Side = iota
	Left
	Right
)

type Chunk struct {
	Start, End        int
	Base, Left, Right string
	Conflict          bool
	Choice            Side
}

type Merge struct {
	Base   []string
	Chunks []Chunk
}

// NewMerge retains line terminators so chunk choices preserve final newlines.
func NewMerge(base, left, right string) *Merge {
	lines := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.SplitAfter(s, "\n")
	}
	b, l, r := lines(base), lines(left), lines(right)
	type edit struct {
		Change
		side Side
	}
	var edits []edit
	for _, c := range Lines(b, l, Default) {
		edits = append(edits, edit{c, Left})
	}
	for _, c := range Lines(b, r, Default) {
		edits = append(edits, edit{c, Right})
	}
	slices.SortStableFunc(edits, func(a, b edit) int {
		if a.StartA != b.StartA {
			return a.StartA - b.StartA
		}
		return a.EndA - b.EndA
	})
	m := &Merge{Base: b}
	for i := 0; i < len(edits); {
		start, end := edits[i].StartA, edits[i].EndA
		j := i + 1
		for j < len(edits) {
			e := edits[j]
			if e.StartA > end || e.StartA == end && e.StartA != e.EndA && start != end {
				break
			}
			end = max(end, e.EndA)
			j++
		}
		part := func(side Side, source []string) (string, bool) {
			var out strings.Builder
			at := start
			found := false
			for _, e := range edits[i:j] {
				if e.side != side {
					continue
				}
				out.WriteString(strings.Join(b[at:e.StartA], ""))
				out.WriteString(strings.Join(source[e.StartB:e.EndB], ""))
				at = e.EndA
				found = true
			}
			out.WriteString(strings.Join(b[at:end], ""))
			return out.String(), found
		}
		lv, hasL := part(Left, l)
		rv, hasR := part(Right, r)
		m.Chunks = append(m.Chunks, Chunk{Start: start, End: end, Base: strings.Join(b[start:end], ""), Left: lv, Right: rv, Conflict: hasL && hasR && lv != rv})
		i = j
	}
	return m
}

func (m *Merge) ApplyNonConflicting() {
	for i := range m.Chunks {
		c := &m.Chunks[i]
		if !c.Conflict {
			if c.Left != c.Base {
				c.Choice = Left
			} else {
				c.Choice = Right
			}
		}
	}
}
func (m *Merge) Remaining() int {
	n := 0
	for _, c := range m.Chunks {
		if c.Choice == Unresolved {
			n++
		}
	}
	return n
}
func (m *Merge) Result() string {
	var out strings.Builder
	at := 0
	for _, c := range m.Chunks {
		out.WriteString(strings.Join(m.Base[at:c.Start], ""))
		switch c.Choice {
		case Left:
			out.WriteString(c.Left)
		case Right:
			out.WriteString(c.Right)
		default:
			out.WriteString(c.Base)
		}
		at = c.End
	}
	out.WriteString(strings.Join(m.Base[at:], ""))
	return out.String()
}
