package branches

import (
	"github.com/runbrick/r-git/internal/i18n"
	"strings"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/vivaui/viva/ui"
)

// StashAction is what the stash panel asked the shell to run.
type StashAction uint8

const (
	StashNone StashAction = iota
	StashApply
	StashPop
	StashDrop
	StashCreate
)

// StashRequest is one stash command.
type StashRequest struct {
	Action  StashAction
	Ref     string
	Message string
	Options ops.StashSaveOptions
}

// StashPanel lists stash entries and creates new ones.
type StashPanel struct {
	dropText i18n.NameText
	popText  i18n.NameText

	i18n.Locale
	Open             bool
	Entries          []model.Stash
	Selected         int
	Message          string
	IncludeUntracked bool
	KeepIndex        bool
	Error            string
	confirmDrop      bool
	confirmPop       bool
	ready            StashRequest
}

// Show opens the panel with the current stash list.
func (p *StashPanel) Show(entries []model.Stash) {
	p.Open = true
	p.Entries = append([]model.Stash(nil), entries...)
	p.Selected = 0
	p.Error = ""
	p.confirmDrop = false
	p.confirmPop = false
	p.ready = StashRequest{}
}

// Page paints the panel and returns a request when the user confirmed one.
func (p *StashPanel) Page(c ui.C) StashRequest {
	var pending StashRequest
	if !p.Open {
		ready := p.ready
		p.ready = StashRequest{}
		return ready
	}
	ui.ModalWith(c, &p.Open, p.Text().Stashes, ui.ModalSpec{NoClose: p.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).FillWidth().Children(func() {
			if len(p.Entries) == 0 {
				ui.Text(c, p.Text().NoStashes).Muted()
			}
			for i, entry := range p.Entries {
				item := entry
				index := i
				label := item.Ref + "  " + item.Message
				ui.Row(c.Key(item.Ref)).FillWidth().Gap(8).Items(ui.Center).Children(func() {
					button := ui.Button(c, label).Ghost()
					if p.Selected == index {
						button.Bold()
					}
					if button.Clicked() {
						p.Selected = index
					}
				})
			}
			ui.Text(c, p.Text().CreateStash).FontSize(12).Muted()
			field := ui.TextField(c, &p.Message, p.Text().Message, "WIP")
			if field.Submitted() {
				pending = StashRequest{
					Action: StashCreate,
					Options: ops.StashSaveOptions{
						Message:          strings.TrimSpace(p.Message),
						IncludeUntracked: p.IncludeUntracked,
						KeepIndex:        p.KeepIndex,
					},
				}
			}
			ui.Checkbox(c, &p.IncludeUntracked, p.Text().IncludeUntracked)
			ui.Checkbox(c, &p.KeepIndex, p.Text().KeepIndex)
			if p.Error != "" {
				ui.Text(c, p.Error).Color(c.Theme().Danger).FontSize(12)
			}
		})
	}, func() {
		if ui.Button(c, p.Text().CloseStash).Outline().Clicked() {
			p.Open = false
		}
		if ui.Button(c, p.Text().SaveStash).Clicked() {
			pending = StashRequest{Action: StashCreate, Options: ops.StashSaveOptions{
				Message: strings.TrimSpace(p.Message), IncludeUntracked: p.IncludeUntracked, KeepIndex: p.KeepIndex,
			}}
		}
		if len(p.Entries) == 0 {
			return
		}
		if p.Selected < 0 || p.Selected >= len(p.Entries) {
			return
		}
		ref := p.Entries[p.Selected].Ref
		if ui.Button(c, p.Text().Apply).Clicked() {
			pending = StashRequest{Action: StashApply, Ref: ref}
		}
		if ui.Button(c, p.Text().Pop).Clicked() {
			p.confirmPop = true
		}
		if ui.Button(c, p.Text().Drop).Clicked() {
			p.confirmDrop = true
		}
	})
	p.confirm(c, &pending)
	if pending.Action != StashNone {
		return pending
	}
	return StashRequest{}
}

func (p *StashPanel) confirm(c ui.C, pending *StashRequest) {
	if p.confirmDrop {
		ref := p.selectedRef()
		ui.ModalWith(c, &p.confirmDrop, p.Text().DropStash, ui.ModalSpec{NoClose: p.Text() != &i18n.En}, func() {
			ui.Text(c, p.dropText.Get(p.Text(), ref, p.Text().DropStashPrompt))
		}, func() {
			if ui.Button(c, p.Text().Cancel).Outline().Clicked() {
				p.confirmDrop = false
			}
			if ui.Button(c, p.Text().Drop).Clicked() {
				p.confirmDrop = false
				*pending = StashRequest{Action: StashDrop, Ref: ref}
			}
		})
	}
	if p.confirmPop {
		ref := p.selectedRef()
		ui.ModalWith(c, &p.confirmPop, p.Text().PopStash, ui.ModalSpec{NoClose: p.Text() != &i18n.En}, func() {
			ui.Text(c, p.popText.Get(p.Text(), ref, p.Text().PopStashPrompt))
		}, func() {
			if ui.Button(c, p.Text().Cancel).Outline().Clicked() {
				p.confirmPop = false
			}
			if ui.Button(c, p.Text().PopNow).Clicked() {
				p.confirmPop = false
				*pending = StashRequest{Action: StashPop, Ref: ref}
			}
		})
	}
}

// Ready is a request recorded outside a click, such as Enter in the message field.
func (p *StashPanel) Ready() StashRequest { return p.ready }

// TakeReady returns and clears that request.
func (p *StashPanel) TakeReady() StashRequest {
	ready := p.ready
	p.ready = StashRequest{}
	return ready
}
func (p *StashPanel) SubmitCreate() {
	p.ready = StashRequest{
		Action: StashCreate,
		Options: ops.StashSaveOptions{
			Message:          strings.TrimSpace(p.Message),
			IncludeUntracked: p.IncludeUntracked,
			KeepIndex:        p.KeepIndex,
		},
	}
}

func (p *StashPanel) selectedRef() string {
	if p.Selected < 0 || p.Selected >= len(p.Entries) {
		return ""
	}
	return p.Entries[p.Selected].Ref
}

// RemoteEdit is one row of the remotes dialog.
type RemoteEdit struct {
	Name    string
	URL     string
	NewName string
	Remove  bool
}

// RemoteChange is one edit the dialog confirmed.
type RemoteChange struct {
	Add    bool
	Remove bool
	Rename bool
	Old    string
	Name   string
	URL    string
}

// RemotesDialog adds, removes, renames, and edits remote URLs.
type RemotesDialog struct {
	i18n.Locale
	Open     bool
	Remotes  []RemoteEdit
	NewName  string
	NewURL   string
	Error    string
	Changes  []RemoteChange
	Accepted bool
}

// Show opens the dialog with the current remotes.
func (d *RemotesDialog) Show(remotes []model.Remote) {
	d.Open = true
	d.Remotes = d.Remotes[:0]
	for _, remote := range remotes {
		d.Remotes = append(d.Remotes, RemoteEdit{Name: remote.Name, URL: remote.FirstURL(), NewName: remote.Name})
	}
	d.NewName = ""
	d.NewURL = ""
	d.Error = ""
	d.Changes = nil
	d.Accepted = false
}

// Page paints the dialog. Accepted is true on the paint that confirms it.
func (d *RemotesDialog) Page(c ui.C) {
	if !d.Open {
		return
	}
	ui.ModalWith(c, &d.Open, d.Text().Remotes, ui.ModalSpec{NoClose: d.Text() != &i18n.En}, func() {
		ui.Column(c).Gap(8).FillWidth().Children(func() {
			for i := range d.Remotes {
				remote := &d.Remotes[i]
				ui.Column(c.Key(remote.Name)).Gap(4).FillWidth().Children(func() {
					ui.Text(c, remote.Name).Bold()
					ui.TextField(c, &remote.NewName, d.Text().Name, remote.Name)
					ui.TextField(c, &remote.URL, d.Text().URL, "")
					ui.Checkbox(c, &remote.Remove, d.Text().Remove)
				})
			}
			ui.Text(c, d.Text().AddRemote).FontSize(12).Muted()
			ui.TextField(c, &d.NewName, d.Text().NewRemoteName, "origin")
			ui.TextField(c, &d.NewURL, d.Text().NewRemoteURL, "https://example.com/repo.git")
			if d.Error != "" {
				ui.Text(c, d.Error).Color(c.Theme().Danger).FontSize(12)
			}
		})
	}, func() {
		if ui.Button(c, d.Text().Cancel).Outline().Clicked() {
			d.Open = false
		}
		if ui.Button(c, d.Text().Save).Clicked() {
			changes, errText := d.collect()
			if errText != "" {
				d.Error = errText
				return
			}
			d.Changes = changes
			d.Accepted = true
			d.Open = false
		}
	})
}

func (d *RemotesDialog) collect() ([]RemoteChange, string) {
	var changes []RemoteChange
	if name := strings.TrimSpace(d.NewName); name != "" || strings.TrimSpace(d.NewURL) != "" {
		if name == "" || strings.TrimSpace(d.NewURL) == "" {
			return nil, d.Text().ANewRemoteNeedsANameAndAURL
		}
		changes = append(changes, RemoteChange{Add: true, Name: name, URL: strings.TrimSpace(d.NewURL)})
	}
	for _, remote := range d.Remotes {
		if remote.Remove {
			changes = append(changes, RemoteChange{Remove: true, Old: remote.Name, Name: remote.Name})
			continue
		}
		newName := strings.TrimSpace(remote.NewName)
		if newName == "" {
			return nil, d.Text().RemoteNameCannotBeEmpty
		}
		if newName != remote.Name {
			changes = append(changes, RemoteChange{Rename: true, Old: remote.Name, Name: newName, URL: strings.TrimSpace(remote.URL)})
		}
		changes = append(changes, RemoteChange{Old: remote.Name, Name: newName, URL: strings.TrimSpace(remote.URL)})
	}
	return changes, ""
}
