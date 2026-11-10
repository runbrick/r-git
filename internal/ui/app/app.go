package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	logview "github.com/runbrick/r-git/internal/ui/log"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const pageSize = 100

// Model owns the window state and its repository lifecycle.
type Model struct {
	configDir  string
	appConfig  config.AppSettings
	repoConfig config.RepoSettings
	path       string
	pathEntry  string
	filter     string
	branch     string
	branches   []string
	repo       *repo.Repo
	commits    []model.Commit
	graphRows  []int
	graph      *logview.GraphRows
	loading    bool
	done       bool
	err        string
	list       ui.ListState
	selected   model.Hash
	details    []model.FileChange
	message    string
}

// New loads settings and opens initialPath when supplied.
func New(configDir, initialPath string) (*Model, error) {
	settings, err := config.LoadApp(configDir)
	if err != nil {
		return nil, err
	}
	m := &Model{configDir: configDir, appConfig: settings, pathEntry: initialPath, branch: "All branches"}
	if initialPath != "" {
		if err := m.open(initialPath); err != nil {
			m.err = err.Error()
		}
	}
	return m, nil
}

// Run opens the Viva window.
func Run(configDir, initialPath string) error {
	m, err := New(configDir, initialPath)
	if err != nil {
		return err
	}
	return ui.Run(viva.Config{Title: "r-git", Width: 1120, Height: 760}, m.Page)
}

// Page declares the main window.
func (m *Model) Page(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		m.toolbar(c)
		if m.repo == nil {
			m.startPage(c)
			return
		}
		m.logPage(c)
	})
}

func (m *Model) toolbar(c ui.C) {
	theme := c.Theme()
	ui.Row(c).FillWidth().PadX(16).PadY(8).Gap(8).Items(ui.Center).Background(theme.Card).Border(1, theme.Border).Children(func() {
		ui.Text(c, "r-git").Bold()
		if m.repo != nil {
			ui.Text(c, filepath.Base(m.repo.Path)).FontSize(12).Muted()
		}
		ui.Spacer(c)
		ui.Button(c, "Fetch").Disabled(true)
		ui.Button(c, "Pull").Disabled(true)
		ui.Button(c, "Push").Disabled(true)
		ui.Button(c, "Commit").Disabled(true)
		if m.repo != nil && ui.Button(c, "Refresh").Clicked() {
			c.After(func() { m.refresh() })
		}
		if m.repo != nil && ui.Button(c, "Open repository").Clicked() {
			c.After(func() { m.repo = nil })
		}
	})
}

func (m *Model) startPage(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(16).Children(func() {
		ui.Text(c, "Open repository").FontSize(20).Bold()
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			field := ui.TextField(c, &m.pathEntry, "Repository path", "Path to a Git working tree").Grow(1)
			if field.Submitted() || ui.Button(c, "Open").Clicked() {
				path := m.pathEntry
				c.After(func() { m.openFromUI(path) })
			}
		})
		if m.err != "" {
			ui.Text(c, m.err).Color(c.Theme().Danger)
		}
		if len(m.appConfig.Recent) > 0 {
			ui.Text(c, "Recent repositories").FontSize(13).Muted()
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, recent := range m.appConfig.Recent {
					item := recent
					ui.Row(c.Key(item.Path)).FillWidth().Pad(8).Gap(12).Items(ui.Center).Background(c.Theme().Card).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, item.DisplayName).Bold()
							ui.Text(c, item.Path).FontSize(12).Muted()
						})
						if ui.Button(c, "Open").Clicked() {
							c.After(func() { m.openFromUI(item.Path) })
						}
					})
				}
			})
		}
	})
}

func (m *Model) logPage(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Pad(12).Gap(8).Items(ui.Center).Background(theme.Background).Children(func() {
			search := ui.TextField(c, &m.filter, "Filter", "Subject or hash")
			search.Grow(1)
			choices := append([]string{"All branches"}, m.branches...)
			if len(choices) == 1 {
				choices = []string{"All branches"}
			}
			oldBranch := m.branch
			ui.Select(c, &m.branch, choices, nil)
			if oldBranch != m.branch {
				branch := m.branch
				c.After(func() { m.changeBranch(branch) })
			}
		})
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Grow(1).Fill().Children(func() {
				m.list.RowWidth = 900
				ui.Row(c).FillWidth().PadX(12).PadY(6).Gap(12).Background(theme.Card).Children(func() {
					ui.Text(c, "Graph").Width(44).FontSize(11).Muted()
					ui.Text(c, "Subject").Grow(1).FontSize(11).Muted()
					ui.Text(c, "Author").Width(140).FontSize(11).Muted()
					ui.Text(c, "Date").Width(100).FontSize(11).Muted()
					ui.Text(c, "Hash").Width(76).FontSize(11).Muted()
				})
				visible := m.visibleCommits()
				ui.List(c, &m.list, len(visible), func(row ui.ListRow) {
					index := visible[row.Index]
					commit := m.commits[index]
					bg := theme.Background
					if row.Selected() {
						bg = theme.Accent
					}
					row.Element.FillWidth().Height(34).PadX(12).Gap(12).Items(ui.Center).Background(bg).DrawOver(func(d *ui.DrawContext) {
						m.drawGraph(d, m.graphRows[index])
					}).Children(func() {
						ui.Slot(row.C).Width(44).Height(28)
						ui.Column(row.C).Grow(1).Gap(2).Children(func() {
							ui.Row(row.C).Gap(6).Items(ui.Center).Children(func() {
								for _, label := range m.labels(commit.Hash) {
									ui.Text(row.C, label).FontSize(10).Bold().Color(theme.AccentForeground).Background(theme.Accent).PadX(5).PadY(2).Radius(theme.Metrics.RadiusControl)
								}
								ui.Text(row.C, commit.Subject).FontSize(13)
							})
						})
						ui.Text(row.C, commit.Author.Name).Width(140).FontSize(12).Muted()
						ui.Text(row.C, relative(commit.Author.When)).Width(100).FontSize(12).Muted()
						ui.Text(row.C, commit.Hash.Short()).Width(76).FontSize(11).Muted().Mono()
					})
				})
				if m.list.Changed() {
					if index := m.list.Selected(); index >= 0 && index < len(visible) {
						m.selectCommit(m.commits[visible[index]].Hash)
					}
				}
				if selected := m.list.Selected(); selected >= 0 && selected < len(visible) && !m.commits[visible[selected]].Hash.Equal(m.selected) {
					m.selectCommit(m.commits[visible[selected]].Hash)
				}
				_, last := m.list.Visible()
				if last >= len(visible)-10 && !m.done && !m.loading {
					c.After(m.loadNext)
				}
				ui.Text(c, fmt.Sprintf("%d commits", len(m.commits))).FillWidth().PadX(12).PadY(5).FontSize(11).Muted()
				if m.err != "" {
					ui.Text(c, m.err).PadX(12).FontSize(12).Color(theme.Danger)
				}
			})
			ui.Divider(c)
			m.detailPane(c)
		})
	})
}

func (m *Model) detailPane(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(320).FillHeight().Pad(16).Gap(10).Background(theme.Card).Children(func() {
		commit, ok := m.selectedCommit()
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
			for _, change := range m.details {
				label := change.Status.String() + "  " + change.Path
				if change.OrigPath != "" {
					label += "  <- " + change.OrigPath
				}
				ui.Text(c, label).FontSize(12).Mono()
			}
		})
	})
}

func (m *Model) drawGraph(d *ui.DrawContext, row int) {
	buffer := m.graph.Row(row)
	if buffer == nil {
		return
	}
	colors := d.Theme.Chart
	center := d.Rect.X + 20
	middle := d.Rect.Y + d.Rect.Height/2
	for _, edge := range buffer.Edges {
		color := colors[edge.ColorId%len(colors)]
		x := d.Rect.X + 8 + float32(edge.PositionInCurrentRow)*12
		d.Canvas.Fill(ui.Rect{X: x, Y: d.Rect.Y + 3, Width: 2, Height: d.Rect.Height - 6}, color)
		if edge.HasArrow {
			glyph := "↓"
			if edge.EdgePrintType == 0 {
				glyph = "↑"
			}
			d.Canvas.Label(glyph, ui.Rect{X: x - 4, Y: middle - 8, Width: 10, Height: 16}, gfx.TextStyle{Size: 12, Color: color})
		}
	}
	for _, node := range buffer.Nodes {
		color := colors[node.ColorId%len(colors)]
		x := d.Rect.X + 8 + float32(node.PositionInCurrentRow)*12
		d.Canvas.Rounded(ui.Rect{X: x - 4, Y: middle - 4, Width: 10, Height: 10}, 5, color)
	}
	_ = center
}

func (m *Model) visibleCommits() []int {
	visible := make([]int, 0, len(m.commits))
	for i, commit := range m.commits {
		if m.filter != "" && !strings.Contains(strings.ToLower(commit.Subject+" "+commit.Hash.String()), strings.ToLower(m.filter)) {
			continue
		}
		visible = append(visible, i)
	}
	return visible
}

func (m *Model) labels(hash model.Hash) []string {
	var labels []string
	if !m.repo.HEAD.IsZero() && hash.Equal(m.repo.HEAD) {
		labels = append(labels, "HEAD")
	}
	for _, ref := range m.repo.Branches {
		if ref.Hash.Equal(hash) {
			labels = append(labels, ref.Ref.Name())
		}
	}
	for _, ref := range m.repo.Tags {
		if ref.Hash.Equal(hash) {
			labels = append(labels, ref.Ref.Name())
		}
	}
	return labels
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

func (m *Model) openFromUI(path string) {
	if err := m.open(path); err != nil {
		m.err = err.Error()
	}
}

func (m *Model) open(path string) error {
	runner, err := command.New()
	if err != nil {
		return err
	}
	opened, err := repo.OpenWithExecutor(context.Background(), path, runner)
	if err != nil {
		return err
	}
	repoSettings, err := config.LoadRepo(m.configDir, opened.Path)
	if err != nil {
		return err
	}
	m.repo, m.repoConfig, m.path = opened, repoSettings, opened.Path
	m.pathEntry, m.err, m.branch = opened.Path, "", "All branches"
	m.branches = []string{}
	for _, branch := range opened.Branches {
		if branch.Ref.Kind() == model.KindLocalBranch {
			m.branches = append(m.branches, branch.Ref.Name())
		}
	}
	m.commits, m.list = nil, ui.ListState{}
	m.done = false
	if err := m.appConfig.TouchRecent(opened.Path, filepath.Base(opened.Path), time.Now().UnixMilli()); err == nil {
		_ = config.SaveApp(m.configDir, m.appConfig)
	}
	m.loadNext()
	return nil
}

func (m *Model) refresh() {
	if m.repo == nil {
		return
	}
	opened, err := repo.Open(context.Background(), m.repo.Path)
	if err != nil {
		m.err = err.Error()
		return
	}
	m.repo = opened
	m.commits, m.list = nil, ui.ListState{}
	m.done, m.err = false, ""
	m.loadNext()
}

func (m *Model) loadNext() {
	if m.repo == nil || m.loading || m.done {
		return
	}
	m.loading = true
	query := repo.LogQuery{MaxCount: pageSize, Skip: len(m.commits)}
	if m.branch != "" && m.branch != "All branches" {
		query.Branch = m.branch
	}
	commits, err := m.repo.Log(context.Background(), query)
	m.loading = false
	if err != nil {
		m.err = err.Error()
		return
	}
	m.commits = append(m.commits, commits...)
	m.done = len(commits) < pageSize
	ordered, err := m.repo.ParentsOrdered(context.Background(), query.Branch)
	if err != nil {
		m.err = err.Error()
		return
	}
	m.graph = logview.NewGraphRows(ordered)
	graphIndex := make(map[string]int, len(ordered))
	for i, item := range ordered {
		graphIndex[item.Commit.String()] = i
	}
	m.graphRows = m.graphRows[:0]
	for _, commit := range m.commits {
		m.graphRows = append(m.graphRows, graphIndex[commit.Hash.String()])
	}
	if m.selected.IsZero() && len(m.commits) > 0 {
		m.list.Select(0)
		m.selectCommit(m.commits[0].Hash)
	}
}

func (m *Model) changeBranch(branch string) {
	if m.repo == nil {
		return
	}
	m.branch = branch
	m.commits, m.graphRows = nil, nil
	m.graph = nil
	m.list = ui.ListState{}
	m.selected = model.Hash{}
	m.done = false
	m.loadNext()
}

func (m *Model) selectCommit(hash model.Hash) {
	m.selected = hash
	if m.repo != nil {
		m.details, _ = m.repo.ChangedFiles(context.Background(), hash)
		if commit, ok := m.find(hash); ok {
			m.message = commit.Message
		}
	}
}

func (m *Model) find(hash model.Hash) (model.Commit, bool) {
	for _, commit := range m.commits {
		if commit.Hash.Equal(hash) {
			return commit, true
		}
	}
	return model.Commit{}, false
}

func (m *Model) selectedCommit() (model.Commit, bool) { return m.find(m.selected) }
