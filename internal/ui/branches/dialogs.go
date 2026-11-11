package branches

import (
	"github.com/runbrick/r-git/internal/i18n"
	"strings"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/vivaui/viva/ui"
)

// CheckoutDialog offers d.Text().StashAndCheckout when checkout fails because
// local changes would be overwritten. The typed error's file list is shown.
type CheckoutDialog struct {
	titleText  i18n.NameText
	promptText i18n.NameText

	i18n.Locale
	Open      bool
	Branch    string
	Files     []string
	Confirmed bool
	Cancelled bool
}

// ShowLocalChanges opens the dialog for a failed checkout of branch.
func (d *CheckoutDialog) ShowLocalChanges(branch string, files []string) {
	d.Open = true
	d.Branch = branch
	d.Files = append([]string(nil), files...)
	d.Confirmed = false
	d.Cancelled = false
}

// Page paints the dialog. Confirmed is true for the paint that accepts it.
func (d *CheckoutDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	title := d.titleText.Get(d.Text(), d.Branch, d.Text().CheckoutTitle)
	ui.ModalWith(c, &d.Open, title, ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, d.Text().LocalChangesWouldBeOverwrittenByCheckout)
			for _, file := range d.Files {
				ui.Text(c, file).FontSize(12).Muted()
			}
			ui.Text(c, d.promptText.Get(d.Text(), d.Branch, d.Text().StashCheckoutPrompt))
		})
	}, func() {
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
			d.Cancelled = true
		}
		if ui.Button(c, d.Text().StashAndCheckout).Clicked() {
			d.Open = false
			d.Confirmed = true
		}
	})
}

// NewBranchDialog creates a branch from a start point.
type NewBranchDialog struct {
	fromText i18n.NameText

	i18n.Locale
	Open       bool
	Name       string
	StartPoint string
	Error      string
	Confirmed  bool
}

// Show opens the dialog with startPoint as the branch the new one starts from.
func (d *NewBranchDialog) Show(startPoint string) {
	d.Open = true
	d.Name = ""
	d.StartPoint = startPoint
	d.Error = ""
	d.Confirmed = false
}

// Page paints the dialog.
func (d *NewBranchDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	ui.ModalWith(c, &d.Open, d.Text().NewBranch, ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).FillWidth().Children(func() {
			ui.Text(c, d.fromText.Get(d.Text(), d.StartPoint, d.Text().FromRef)).FontSize(12).Muted()
			field := ui.TextField(c, &d.Name, d.Text().BranchName, "feature")
			if field.Submitted() {
				d.confirm()
			}
			if d.Error != "" {
				ui.Text(c, d.Error).Color(c.Theme().Danger).FontSize(12)
			}
		})
	}, func() {
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
		}
		if ui.Button(c, d.Text().CreateBranch).Clicked() {
			d.confirm()
		}
	})
}

func (d *NewBranchDialog) confirm() {
	if strings.TrimSpace(d.Name) == "" {
		d.Error = d.Text().EnterABranchName
		return
	}
	d.Confirmed = true
	d.Open = false
}

// RenameDialog renames a local branch.
type RenameDialog struct {
	titleText i18n.NameText

	i18n.Locale
	Open      bool
	Old       string
	Name      string
	Confirmed bool
}

// Show opens the dialog for old.
func (d *RenameDialog) Show(old string) {
	d.Open = true
	d.Old = old
	d.Name = old
	d.Confirmed = false
}

// Page paints the dialog.
func (d *RenameDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	ui.ModalWith(c, &d.Open, d.titleText.Get(d.Text(), d.Old, d.Text().RenameTitle), ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.TextField(c, &d.Name, d.Text().NewName, "")
	}, func() {
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
		}
		if ui.Button(c, d.Text().Rename).Clicked() && strings.TrimSpace(d.Name) != "" {
			d.Confirmed = true
			d.Open = false
		}
	})
}

// DeleteDialog confirms deletion. Unmerged is set when git refused a
// non-force delete; confirming then force-deletes.
type DeleteDialog struct {
	titleText  i18n.NameText
	promptText i18n.NameText

	i18n.Locale
	Open      bool
	Branch    string
	Unmerged  *ops.ErrNotFullyMerged
	Confirmed bool
	Force     bool
}

// Show opens a plain delete confirmation.
func (d *DeleteDialog) Show(branch string) {
	d.Open = true
	d.Branch = branch
	d.Unmerged = nil
	d.Confirmed = false
	d.Force = false
}

// ShowUnmerged opens the dialog Java shows when the branch is not fully merged.
func (d *DeleteDialog) ShowUnmerged(err *ops.ErrNotFullyMerged) {
	d.Open = true
	d.Branch = err.Branch
	d.Unmerged = err
	d.Confirmed = false
	d.Force = false
}

// Page paints the dialog.
func (d *DeleteDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	ui.ModalWith(c, &d.Open, d.titleText.Get(d.Text(), d.Branch, d.Text().DeleteTitle), ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).Children(func() {
			if d.Unmerged == nil {
				ui.Text(c, d.promptText.Get(d.Text(), d.Branch, d.Text().DeleteBranchPrompt))
				return
			}
			ui.Text(c, d.Text().NotFullyMerged(d.Unmerged.Branch, d.Unmerged.Base))
			ui.Text(c, d.Text().UnmergedCount(len(d.Unmerged.Commits))).FontSize(12).Muted()
		})
	}, func() {
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
		}
		label := d.Text().Delete
		if d.Unmerged != nil {
			label = d.Text().ForceDelete
		}
		if ui.Button(c, label).Clicked() {
			d.Confirmed = true
			d.Force = d.Unmerged != nil
			d.Open = false
		}
	})
}

// PushTarget is one local branch the push dialog can send.
type PushTarget struct {
	Branch   string
	Remote   string
	Target   string
	Selected bool
	Commits  []model.Commit
}

// PushDialog lists outgoing commits per branch and collects push options.
type PushDialog struct {
	i18n.Locale
	Open       bool
	Targets    []PushTarget
	ForceLease bool
	PushTags   bool
	AllowForce bool
	Error      string
	Confirmed  bool
	// PullThenPush is set when the user accepts the rejected-push recovery.
	PullThenPush bool
	Results      []ops.PushNativeResult
	ShowResults  bool
}

// Show opens the dialog with one row per local branch that has outgoing commits.
func (d *PushDialog) Show(targets []PushTarget, allowForce bool) {
	d.Open = true
	d.Targets = append([]PushTarget(nil), targets...)
	d.ForceLease = false
	d.PushTags = false
	d.AllowForce = allowForce
	d.Error = ""
	d.Confirmed = false
	d.PullThenPush = false
	d.Results = nil
	d.ShowResults = false
}

// ShowRejected opens the per-ref result view and offers d.Text().PullThenPush
// when a rejection is a non-fast-forward.
func (d *PushDialog) ShowRejected(results []ops.PushNativeResult) {
	d.Open = true
	d.Results = append([]ops.PushNativeResult(nil), results...)
	d.ShowResults = true
	d.Confirmed = false
	d.PullThenPush = false
}

// Page paints the dialog.
func (d *PushDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	title := d.Text().Push
	if d.ShowResults {
		title = d.Text().PushResults
	}
	ui.ModalWith(c, &d.Open, title, ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).FillWidth().Children(func() {
			if d.ShowResults {
				d.results(c)
				return
			}
			d.form(c)
		})
	}, func() {
		if d.ShowResults {
			if ui.Button(c, d.Text().Close).Outline().Clicked() {
				d.Open = false
			}
			if RejectedNonFF(d.Results) && ui.Button(c, d.Text().PullThenPush).Clicked() {
				d.PullThenPush = true
				d.Open = false
			}
			return
		}
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
		}
		if ui.Button(c, d.Text().PushCommits).Clicked() {
			if d.selectedCount() == 0 {
				d.Error = d.Text().SelectABranchToPush
				return
			}
			d.Confirmed = true
			d.Open = false
		}
	})
}

func (d *PushDialog) form(c ui.C) {
	if len(d.Targets) == 0 {
		ui.Text(c, d.Text().NothingToPush)
		return
	}
	for i := range d.Targets {
		target := &d.Targets[i]
		ui.Column(c.Key(target.Branch)).Gap(4).FillWidth().Children(func() {
			box := ui.Checkbox(c, &d.Targets[i].Selected, d.Targets[i].Branch)
			// The click settles into the bool after this function returns, and a
			// range variable's address is not that bool. Read the click here.
			if box.Clicked() {
				d.Targets[i].Selected = !d.Targets[i].Selected
			}
			ui.Row(c).Gap(8).Children(func() {
				ui.TextField(c, &target.Remote, d.Text().Remote, "origin").Grow(1)
				ui.TextField(c, &target.Target, d.Text().TargetBranch, target.Branch).Grow(1)
			})
			if len(target.Commits) == 0 {
				ui.Text(c, d.Text().NoOutgoingCommits).FontSize(12).Muted()
			}
			for _, commit := range target.Commits {
				ui.Text(c, commit.Subject).FontSize(12)
			}
		})
	}
	if d.AllowForce {
		ui.Checkbox(c, &d.ForceLease, d.Text().ForceWithLease)
	}
	ui.Checkbox(c, &d.PushTags, d.Text().PushTags)
	if d.Error != "" {
		ui.Text(c, d.Error).Color(c.Theme().Danger).FontSize(12)
	}
}

func (d *PushDialog) results(c ui.C) {
	if len(d.Results) == 0 {
		ui.Text(c, d.Text().PushProducedNoPerRefResults)
		return
	}
	for i, result := range d.Results {
		ui.Text(c.KeyInt(i), d.resultLine(result)).FontSize(12)
	}
	if RejectedNonFF(d.Results) {
		ui.Text(c, d.Text().ABranchWasRejectedPullThenPushAgain)
	}
}

func (d *PushDialog) selectedCount() int {
	n := 0
	for _, target := range d.Targets {
		if target.Selected {
			n++
		}
	}
	return n
}

func (d *PushDialog) resultLine(result ops.PushNativeResult) string {
	types := [...]string{d.Text().PushSuccess, d.Text().PushForced, d.Text().PushNew, d.Text().PushRejected, d.Text().PushDeleted, d.Text().PushUpToDate, d.Text().PushError}
	kind := d.Text().PushError
	if int(result.Type) >= 0 && int(result.Type) < len(types) {
		kind = types[result.Type]
	}
	text := kind + " " + result.SourceRef
	if result.Reason != "" {
		text += " (" + result.Reason + ")"
	}
	return text
}

// PullDialog collects remote, branch, and merge or rebase.
type PullDialog struct {
	i18n.Locale
	Open      bool
	Remote    string
	Branch    string
	Method    config.UpdateMethod
	Confirmed bool
	Error     string
}

// Show opens the dialog. method is the repository setting.
func (d *PullDialog) Show(remote, branch string, method config.UpdateMethod) {
	d.Open = true
	d.Remote = remote
	d.Branch = branch
	if method == "" {
		method = config.UpdateMerge
	}
	d.Method = method
	d.Confirmed = false
	d.Error = ""
}

// Page paints the dialog.
func (d *PullDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	ui.ModalWith(c, &d.Open, d.Text().Pull, ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).FillWidth().Children(func() {
			ui.TextField(c, &d.Remote, d.Text().Remote, "origin")
			ui.TextField(c, &d.Branch, d.Text().Branch, "")
			ui.Text(c, d.Text().UpdateMethod).FontSize(12).Muted()
			merge := ui.Button(c, d.Text().Merge)
			if d.Method == config.UpdateMerge {
				merge.Bold()
			}
			if merge.Clicked() {
				d.Method = config.UpdateMerge
			}
			rebase := ui.Button(c, d.Text().Rebase)
			if d.Method == config.UpdateRebase {
				rebase.Bold()
			}
			if rebase.Clicked() {
				d.Method = config.UpdateRebase
			}
			if d.Error != "" {
				ui.Text(c, d.Error).Color(c.Theme().Danger).FontSize(12)
			}
		})
	}, func() {
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
		}
		if ui.Button(c, d.Text().PullNow).Clicked() {
			if strings.TrimSpace(d.Remote) == "" {
				d.Error = d.Text().EnterARemote
				return
			}
			d.Confirmed = true
			d.Open = false
		}
	})
}

// MethodOf maps the dialog choice onto the puller. Branch-default follows
// the branch's own rebase flag from tracking info.
func MethodOf(method config.UpdateMethod, rebase bool) ops.PullMethod {
	switch method {
	case config.UpdateRebase:
		return ops.PullMethodRebase
	case config.UpdateBranchDefault:
		if rebase {
			return ops.PullMethodRebase
		}
		return ops.PullMethodMerge
	default:
		return ops.PullMethodMerge
	}
}
