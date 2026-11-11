package branches

import (
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/model"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestDialogsDismissAndStayClosed(t *testing.T) {
	for _, dialog := range []string{"stash", "push", "pull", "remotes", "checkout", "new", "rename", "delete"} {
		for _, action := range []string{"escape", "Close dialog", "cancel"} {
			t.Run(dialog+"/"+action, func(t *testing.T) {
				var paint func(ui.C)
				var isOpen func() bool
				cancel := "Cancel"
				switch dialog {
				case "stash":
					d := &StashPanel{}
					d.Show(nil)
					paint = func(c ui.C) { d.Page(c) }
					isOpen = func() bool { return d.Open }
					cancel = "Close stash"
				case "push":
					d := &PushDialog{}
					d.Show(nil, false)
					paint = d.Page
					isOpen = func() bool { return d.Open }
				case "pull":
					d := &PullDialog{}
					d.Show("origin", "main", config.UpdateMerge)
					paint = d.Page
					isOpen = func() bool { return d.Open }
				case "remotes":
					d := &RemotesDialog{}
					d.Show([]model.Remote{{Name: "origin"}})
					paint = d.Page
					isOpen = func() bool { return d.Open }
				case "checkout":
					d := &CheckoutDialog{}
					d.ShowLocalChanges("topic", []string{"file.txt"})
					paint = d.Page
					isOpen = func() bool { return d.Open }
				case "new":
					d := &NewBranchDialog{}
					d.Show("main")
					paint = d.Page
					isOpen = func() bool { return d.Open }
				case "rename":
					d := &RenameDialog{}
					d.Show("main")
					paint = d.Page
					isOpen = func() bool { return d.Open }
				case "delete":
					d := &DeleteDialog{}
					d.Show("topic")
					paint = d.Page
					isOpen = func() bool { return d.Open }
				}
				h, err := headless.Open(ui.NewPage(ui.PageConfig{View: paint}).Config(viva.Config{Width: 1000, Height: 800}), nil, headless.Options{})
				if err != nil {
					t.Fatal(err)
				}
				defer h.Close()
				if action == "escape" {
					err = h.Keys("escape")
				} else if action == "cancel" {
					err = h.ClickLabel(cancel)
				} else {
					err = h.ClickLabel(action)
				}
				if err != nil {
					t.Fatal(err)
				}
				if isOpen() {
					t.Fatal("dialog still open")
				}
				// Ordinary frames must not reassert an open flag after a dismissal.
				if err = h.Wait(time.Second); err != nil {
					t.Fatal(err)
				}
				if isOpen() {
					t.Fatal("dialog reopened")
				}
				for _, hit := range h.Snapshot().Hits {
					if hit.Label == "Close dialog" {
						t.Fatal("dismissed dialog still takes input")
					}
				}
			})
		}
	}
}

// Minimal repro of the headless observation behind PR8's dismissal report:
// input targets disappear when closed, but raw Texts can include the exiting
// modal until its motion completes. Reading Texts does not advance that clock.
func TestVivaModalDismissalDropsHitsBeforeExitAnimation(t *testing.T) {
	for _, action := range []string{"escape", "Close dialog", "Close body"} {
		t.Run(action, func(t *testing.T) {
			open := false
			page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
				if ui.Button(c, "Open").Clicked() {
					open = true
				}
				ui.Modal(c, &open, "Minimal modal", func() {
					if ui.Button(c, "Close body").Clicked() {
						open = false
					}
				}, nil)
			}})
			h, err := headless.Open(page.Config(viva.Config{Width: 800, Height: 600}), nil, headless.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			if err = h.ClickLabel("Open"); err != nil {
				t.Fatal(err)
			}
			if err = h.Wait(50 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if action == "escape" {
				err = h.Keys(action)
			} else {
				err = h.ClickLabel(action)
			}
			if err != nil {
				t.Fatal(err)
			}
			if open {
				t.Fatal("open flag was not cleared")
			}
			for _, hit := range h.Snapshot().Hits {
				if hit.Label == "Close body" {
					t.Fatal("dismissed modal takes input")
				}
			}
			fadingText := false
			for _, text := range h.Snapshot().Texts {
				if text.Value == "Minimal modal" {
					fadingText = true
				}
			}
			if !fadingText {
				t.Fatal("repro requires the modal's raw text during its exit animation")
			}
			if err = h.WaitFor(func(snapshot viva.Snapshot) bool {
				for _, text := range snapshot.Texts {
					if text.Value == "Minimal modal" {
						return false
					}
				}
				return true
			}, time.Second); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStashOperationResultDoesNotReopenDismissedDialog(t *testing.T) {
	var session Session
	session.Stash.Show(nil)
	session.Stash.Open = false
	session.ApplyUI(UIEvent{UpdateStash: true, Stashes: []model.Stash{{Ref: "stash@{0}", Message: "saved work"}}})
	if session.Stash.Open || len(session.Stash.Entries) != 1 {
		t.Fatalf("late result: %+v", session.Stash)
	}
}

func TestSidePanelActionsHaveDistinctLabelsAndDispatch(t *testing.T) {
	for _, tc := range []struct {
		label  string
		action Action
	}{
		{"Fetch remote", ActionFetch}, {"Pull changes", ActionPull}, {"Push branches", ActionPush}, {"Stash", ActionStash}, {"Remotes", ActionRemotes},
	} {
		t.Run(tc.label, func(t *testing.T) {
			var got Request
			panel := Panel{OnRequest: func(req Request) { got = req }}
			page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { panel.Page(c) }})
			h, err := headless.Open(page.Config(viva.Config{Width: 320, Height: 800}), nil, headless.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			if err = h.ClickLabel(tc.label); err != nil {
				t.Fatal(err)
			}
			if got.Action != tc.action {
				t.Fatalf("action %v want %v", got.Action, tc.action)
			}
		})
	}
}
