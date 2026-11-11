package log

import (
	"fmt"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// View owns the virtualized history table, filters, graph and selection.
type View struct {
	Commits      []model.Commit
	Graph        *GraphRows
	GraphIndex   map[model.Hash]int
	Refs         map[model.Hash][]string
	Head         model.Hash
	Details      []model.FileChange
	Filter       string
	Branch       string
	Branches     []string
	Choices      []string
	Loading      bool
	Done         bool
	Error        string
	OnBranch     func(string)
	OnPage       func(string, int)
	OnSelect     func(model.Hash)
	OnGraphBuilt func()
	OnShowDiff   func(model.Hash, string)

	list             ui.ListState
	visible          []int
	searchText       []string
	normalizedFilter string
	selected         model.Hash
}

// SetRefs caches ref labels once per repository snapshot.
func (v *View) SetRefs(head model.Hash, branches, tags []repo.Reference) {
	v.Head = head
	v.Refs = make(map[model.Hash][]string, len(branches)+len(tags)+1)
	for _, ref := range branches {
		v.Refs[ref.Hash] = append(v.Refs[ref.Hash], ref.Ref.Name())
	}
	for _, ref := range tags {
		v.Refs[ref.Hash] = append(v.Refs[ref.Hash], ref.Ref.Name())
	}
	if !head.IsZero() {
		v.Refs[head] = append([]string{"HEAD"}, v.Refs[head]...)
	}
}

// SetTopology prepares the graph exactly once for a repository/branch history.
func (v *View) SetTopology(ordered []repo.CommitParents) error {
	graphRows, indexes, err := NewGraphRows(ordered)
	if err != nil {
		return err
	}
	v.SetGraph(graphRows, indexes)
	return nil
}

// SetGraph installs a graph prepared outside the UI thread.
func (v *View) SetGraph(graphRows *GraphRows, indexes map[model.Hash]int) {
	v.Graph, v.GraphIndex = graphRows, indexes
	if v.OnGraphBuilt != nil {
		v.OnGraphBuilt()
	}
	v.rebuildVisible()
}

// AppendPage adds one metadata page and refreshes the filter index.
func (v *View) AppendPage(commits []model.Commit, done bool) {
	for i := range commits {
		v.searchText = append(v.searchText, strings.ToLower(commits[i].Subject+" "+commits[i].Hash.String()))
	}
	v.Commits = append(v.Commits, commits...)
	v.Done, v.Loading = done, false
	v.rebuildVisible()
}

// SetError ends loading and exposes an error in the view.
func (v *View) SetError(err error) {
	v.Loading = false
	if err != nil {
		v.Error = err.Error()
	}
}

// Select sets the selected commit after the first metadata page arrives.
func (v *View) Select(hash model.Hash) {
	v.selected = hash
	for i, commit := range v.Commits {
		if commit.Hash.Equal(hash) {
			v.list.Select(i)
			return
		}
	}
}

// ResetBranch clears branch-scoped commit state while retaining reusable list storage.
func (v *View) ResetBranch(branch string) {
	v.Branch, v.Commits, v.Details = branch, nil, nil
	v.searchText = v.searchText[:0]
	v.Graph, v.GraphIndex = nil, nil
	v.Done, v.Loading, v.Error = false, true, ""
	v.selected = model.Hash{}
	v.list = ui.ListState{}
	v.rebuildVisible()
}

// Selected returns the selected commit hash.
func (v *View) Selected() model.Hash { return v.selected }

// VisibleCount returns the cached text-filter result count.
func (v *View) VisibleCount() int { return len(v.visible) }

// VisibleRange returns the last measured visible row indices.
func (v *View) VisibleRange() (int, int) { return v.list.Visible() }

// VisibleCommit returns the filtered commit at a displayed row.
func (v *View) VisibleCommit(row int) (model.Commit, bool) {
	if row < 0 || row >= len(v.visible) {
		return model.Commit{}, false
	}
	return v.Commits[v.visible[row]], true
}

// SetFilter updates the filter and its cached visible index.
func (v *View) SetFilter(filter string) {
	if v.Filter != filter {
		v.Filter = filter
		v.rebuildVisible()
	}
}

func (v *View) rebuildVisible() {
	v.normalizedFilter = strings.ToLower(strings.TrimSpace(v.Filter))
	v.visible = v.visible[:0]
	for i := range v.Commits {
		if v.normalizedFilter != "" && !strings.Contains(v.searchText[i], v.normalizedFilter) {
			continue
		}
		v.visible = append(v.visible, i)
	}
}

// Page declares the log and details panes.
func (v *View) Page(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Pad(12).Gap(8).Items(ui.Center).Background(theme.Background).Children(func() {
			oldFilter, oldBranch := v.Filter, v.Branch
			search := ui.TextField(c, &v.Filter, "Filter", "Subject or hash")
			search.Grow(1)
			ui.Select(c, &v.Branch, v.Choices, nil).Label("Branch")
			if oldFilter != v.Filter {
				v.rebuildVisible()
			}
			if oldBranch != v.Branch && v.OnBranch != nil {
				branch := v.Branch
				c.After(func() { v.OnBranch(branch) })
			}
		})
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Grow(1).Fill().Children(func() {
				v.list.RowWidth = 900
				if v.list.Label == nil {
					v.list.Label = v.rowLabel
				}
				ui.Row(c).FillWidth().PadX(12).PadY(6).Gap(12).Background(theme.Card).Children(func() {
					ui.Text(c, "Graph").Width(44).FontSize(11).Muted()
					ui.Text(c, "Subject").Grow(1).FontSize(11).Muted()
					ui.Text(c, "Author").Width(140).FontSize(11).Muted()
					ui.Text(c, "Date").Width(100).FontSize(11).Muted()
					ui.Text(c, "Hash").Width(76).FontSize(11).Muted()
				})
				ui.List(c, &v.list, len(v.visible), func(row ui.ListRow) {
					index := v.visible[row.Index]
					commit := v.Commits[index]
					bg := theme.Background
					if row.Selected() {
						bg = theme.Accent
					}
					row.Element.FillWidth().Height(34).PadX(12).Gap(12).Items(ui.Center).Background(bg).DrawOver(func(d *ui.DrawContext) {
						v.drawGraph(d, v.GraphIndex[commit.Hash])
					}).Children(func() {
						ui.Slot(row.C).Width(44).Height(28)
						ui.Column(row.C).Grow(1).Gap(2).Children(func() {
							ui.Row(row.C).Gap(6).Items(ui.Center).Children(func() {
								for _, label := range v.Refs[commit.Hash] {
									ui.Text(row.C, label).FontSize(10).Bold().Color(theme.AccentForeground).Background(theme.Accent).PadX(5).PadY(2).Radius(theme.Metrics.RadiusControl)
								}
								ui.Text(row.C, commit.Subject).FontSize(13)
							})
						})
						ui.Text(row.C, commit.Author.Name).Width(140).FontSize(12).Muted()
						ui.Text(row.C, relative(commit.Author.When)).Width(100).FontSize(12).Muted()
						ui.Text(row.C, commit.Hash.Short()).Width(76).FontSize(11).Muted().Mono()
					})
				}).Grow(1)
				if selected := v.list.Selected(); selected >= 0 && selected < len(v.visible) {
					hash := v.Commits[v.visible[selected]].Hash
					if !hash.Equal(v.selected) {
						v.selected = hash
						if v.OnSelect != nil {
							v.OnSelect(hash)
						}
					}
				}
				_, last := v.list.Visible()
				if last >= len(v.visible)-10 && !v.Done && !v.Loading && v.OnPage != nil {
					branch, skip := v.Branch, len(v.Commits)
					v.Loading = true
					v.OnPage(branch, skip)
				}
				ui.Row(c).FillWidth().PadX(12).PadY(5).Gap(8).Items(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("%d commits", len(v.Commits))).FontSize(11).Muted()
					if v.Loading {
						ui.Text(c, "Loading...").FontSize(11).Muted()
					}
					if v.Error != "" {
						ui.Text(c, v.Error).FontSize(12).Color(theme.Danger)
					}
				})
			})
			ui.Divider(c)
			v.detailPane(c)
		})
	})
}

func (v *View) rowLabel(row int) string {
	if row < 0 || row >= len(v.visible) {
		return ""
	}
	return v.Commits[v.visible[row]].Subject
}

func (v *View) detailPane(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(320).FillHeight().Pad(16).Gap(10).Background(theme.Card).Children(func() {
		commit, ok := v.selectedCommit()
		if !ok {
			ui.Text(c, "Select a commit").Muted()
			return
		}
		ui.Text(c, commit.Subject).FontSize(15).Bold()
		ui.Text(c, commit.Message).FontSize(12)
		ui.Divider(c)
		ui.Text(c, "Author").FontSize(11).Muted()
		ui.Text(c, fmt.Sprintf("%s <%s>", commit.Author.Name, commit.Author.Email)).FontSize(12)
		ui.Text(c, "Committer").FontSize(11).Muted()
		ui.Text(c, fmt.Sprintf("%s <%s>", commit.Committer.Name, commit.Committer.Email)).FontSize(12)
		ui.Text(c, "Parents").FontSize(11).Muted()
		parents := make([]string, len(commit.Parents))
		for i, parent := range commit.Parents {
			parents[i] = parent.Short()
		}
		ui.Text(c, strings.Join(parents, "  ")).FontSize(12).Mono()
		ui.Text(c, "Changed files").FontSize(11).Muted()
		ui.Scroll(c).Grow(1).Gap(4).Children(func() {
			for _, change := range v.Details {
				label := change.Status.String() + "  " + change.Path
				if change.OrigPath != "" {
					label += "  <- " + change.OrigPath
				}
				if v.OnShowDiff == nil {
					ui.Text(c, label).FontSize(12).Mono()
				} else if ui.Button(c.Key(change.Path), label).Clicked() {
					hash, path := commit.Hash, change.Path
					c.After(func() { v.OnShowDiff(hash, path) })
				}
			}
		})
	})
}

func (v *View) drawGraph(d *ui.DrawContext, row int) {
	if v.Graph == nil {
		return
	}
	buffer := v.Graph.Row(row)
	if buffer == nil {
		return
	}
	colors := d.Theme.Chart
	middle := d.Rect.Y + d.Rect.Height/2
	for _, edge := range buffer.Edges {
		color := colors[edge.ColorId%len(colors)]
		x := d.Rect.X + 8 + float32(edge.PositionInCurrentRow)*12
		d.Canvas.Fill(gfx.Rect{X: x, Y: d.Rect.Y + 3, Width: 2, Height: d.Rect.Height - 6}, color)
		if edge.HasArrow {
			arrow := "↓"
			if edge.EdgePrintType == 0 {
				arrow = "↑"
			}
			d.Canvas.Label(arrow, gfx.Rect{X: x - 4, Y: middle - 8, Width: 10, Height: 16}, gfx.TextStyle{Size: 12, Color: color})
		}
	}
	for _, node := range buffer.Nodes {
		color := colors[node.ColorId%len(colors)]
		x := d.Rect.X + 8 + float32(node.PositionInCurrentRow)*12
		d.Canvas.Rounded(gfx.Rect{X: x - 4, Y: middle - 4, Width: 10, Height: 10}, 5, color)
	}
}

func relative(when time.Time) string {
	delta := time.Since(when)
	switch {
	case delta < time.Minute:
		return "just now"
	case delta < time.Hour:
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	case delta < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
	default:
		return when.Format("2006-01-02")
	}
}
func (v *View) selectedCommit() (model.Commit, bool) {
	for _, commit := range v.Commits {
		if commit.Hash.Equal(v.selected) {
			return commit, true
		}
	}
	return model.Commit{}, false
}
