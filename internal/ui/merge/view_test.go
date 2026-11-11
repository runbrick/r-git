package merge

import (
	"fmt"
	"strings"
	"testing"

	textdiff "github.com/runbrick/r-git/internal/diff"
	"github.com/runbrick/r-git/internal/git/ops"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestConflictHunkButtonsAreReachableAfterScrolling(t *testing.T) {
	var base, left, right strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&base, "base %d\nkeep %d a\nkeep %d b\nkeep %d c\n", i, i, i, i)
		fmt.Fprintf(&left, "ours %d\nkeep %d a\nkeep %d b\nkeep %d c\n", i, i, i, i)
		fmt.Fprintf(&right, "theirs %d\nkeep %d a\nkeep %d b\nkeep %d c\n", i, i, i, i)
	}
	var view View
	request := uint64(7)
	view.OnEdit = func() uint64 { return request }
	view.SetFile("conflict.txt", ops.ConflictContent{Base: base.String(), Left: left.String(), Right: right.String()})
	if len(view.Merge.Chunks) != 12 {
		t.Fatalf("chunks %d", len(view.Merge.Chunks))
	}
	page := ui.NewPage(ui.PageConfig{View: view.Page})
	d, err := viva.NewDriver(page.Config(viva.Config{Width: 1600, Height: 1000}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	reachable := func(label string) (gfx.Hit, bool) {
		for _, hit := range d.Snapshot().Hits {
			if hit.Label != label {
				continue
			}
			target, ok := d.HitTest(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2)
			if ok && target == hit.ID {
				return hit, true
			}
		}
		return gfx.Hit{}, false
	}
	first, ok := reachable("Accept Theirs 1")
	if !ok {
		t.Fatal("first painted button cannot be clicked")
	}
	if err = d.Click(first.Bounds.X+first.Bounds.Width/2, first.Bounds.Y+first.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if view.Edit != 7 || view.manual {
		t.Fatal("hunk choice must protect against stale loads and allow the next choice")
	}
	if view.Merge.Chunks[0].Choice != textdiff.Right {
		t.Fatal("first button did not choose theirs")
	}
	if _, ok = reachable("Accept Yours 12"); ok {
		t.Fatal("offscreen last button takes input before scrolling")
	}
	request++
	var last gfx.Hit
	for i := 0; i < 20; i++ {
		if last, ok = reachable("Accept Yours 12"); ok {
			break
		}
		if err = d.Scroll(gfx.ScrollEvent{X: first.Bounds.X + first.Bounds.Width/2, Y: first.Bounds.Y + first.Bounds.Height/2, DeltaY: 80}); err != nil {
			t.Fatal(err)
		}
	}
	if !ok {
		t.Fatal("last button cannot be reached by scrolling the hunk list")
	}
	if err = d.Click(last.Bounds.X+last.Bounds.Width/2, last.Bounds.Y+last.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if view.Edit != 8 {
		t.Fatal("later hunk choice did not record current request")
	}
	if view.Merge.Chunks[11].Choice != textdiff.Left {
		t.Fatal("last button did not choose ours")
	}
	if view.Merge.Chunks[0].Choice != textdiff.Right {
		t.Fatal("scroll changed earlier choice")
	}
}
