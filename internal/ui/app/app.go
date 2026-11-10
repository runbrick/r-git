package app

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	logview "github.com/runbrick/r-git/internal/ui/log"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const pageSize = 100

type resultKind uint8

const (
	resultOpen resultKind = iota
	resultPage
	resultDetails
)

type loadResult struct {
	kind       resultKind
	generation uint64
	path       string
	branch     string
	repo       *repo.Repo
	ordered    []repo.CommitParents
	graph      *logview.GraphRows
	graphIndex map[model.Hash]int
	commits    []model.Commit
	details    []model.FileChange
	hash       model.Hash
	done       bool
	err        error
}

// Model owns the application shell and coordinates repository work.
type Model struct {
	configDir  string
	appConfig  config.AppSettings
	repoConfig config.RepoSettings
	initial    string
	pathEntry  string
	repo       *repo.Repo
	log        logview.View
	branches   []string
	choices    []string
	err        string

	OnFetch    func()
	OnPull     func()
	OnPush     func()
	OnCommit   func()
	OnShowDiff func(model.Hash, string)

	invalidator viva.Invalidator
	mu          sync.Mutex
	pending     []*loadResult
	ctx         context.Context
	cancel      context.CancelFunc
	workers     sync.WaitGroup
	generation  uint64
	startQueued bool
	closed      bool
}

// New loads settings. Repository I/O starts after the window's first paint.
func New(configDir, initialPath string) (*Model, error) {
	settings, err := config.LoadApp(configDir)
	if err != nil {
		return nil, err
	}
	m := &Model{configDir: configDir, appConfig: settings, initial: initialPath, pathEntry: initialPath}
	m.log.Branch = "All branches"
	m.log.Choices = []string{"All branches"}
	m.log.Done = initialPath == ""
	m.log.OnBranch = m.changeBranch
	m.log.OnPage = m.loadPage
	m.log.OnSelect = m.loadDetails
	return m, nil
}

// Run opens the Viva window.
func Run(configDir, initialPath string) error {
	m, err := New(configDir, initialPath)
	if err != nil {
		return err
	}
	return ui.Run(m.config(), m.Page)
}

func (m *Model) config() viva.Config {
	return viva.Config{Title: "r-git", Width: 1120, Height: 760, Invalidator: &m.invalidator,
		OnWake:   m.applyPending,
		OnClosed: func() { m.stopWorkers() },
	}
}

// Page composes the toolbar and either the repository picker or log view.
func (m *Model) Page(c ui.C) {
	theme := c.Theme()
	if m.repo == nil && m.initial != "" && !m.startQueued {
		m.startQueued = true
		m.startOpen(m.initial)
	}
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		m.toolbar(c)
		if m.repo == nil {
			m.startPage(c)
			return
		}
		m.log.Page(c)
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
		if m.OnFetch == nil {
			ui.Button(c, "Fetch").Disabled(true)
		} else if ui.Button(c, "Fetch").Clicked() {
			c.After(m.OnFetch)
		}
		if m.OnPull == nil {
			ui.Button(c, "Pull").Disabled(true)
		} else if ui.Button(c, "Pull").Clicked() {
			c.After(m.OnPull)
		}
		if m.OnPush == nil {
			ui.Button(c, "Push").Disabled(true)
		} else if ui.Button(c, "Push").Clicked() {
			c.After(m.OnPush)
		}
		if m.OnCommit == nil {
			ui.Button(c, "Commit").Disabled(true)
		} else if ui.Button(c, "Commit").Clicked() {
			c.After(m.OnCommit)
		}
		if m.repo != nil && ui.Button(c, "Refresh").Clicked() {
			c.After(func() { m.startOpen(m.repo.Path) })
		}
		if m.repo != nil && ui.Button(c, "Open repository").Clicked() {
			c.After(m.returnToPicker)
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
				c.After(func() { m.startOpen(path) })
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
						ui.Column(c).Grow(1).Gap(2).Children(func() { ui.Text(c, item.DisplayName).Bold(); ui.Text(c, item.Path).FontSize(12).Muted() })
						if ui.Button(c, "Open").Clicked() {
							c.After(func() { m.startOpen(item.Path) })
						}
					})
				}
			})
		}
		if m.log.Loading {
			ui.Text(c, "Loading repository...").FontSize(12).Muted()
		}
	})
}

func (m *Model) startOpen(path string) {
	path = filepath.Clean(path)
	m.cancelCurrent()
	generation := m.nextGeneration()
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel, m.log.Loading = ctx, cancel, true
	m.log.Error, m.err = "", ""
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		out := loadResult{kind: resultOpen, generation: generation, path: path}
		runner, err := command.New()
		if err == nil {
			out.repo, err = repo.OpenWithExecutor(ctx, path, runner)
		}
		if err == nil {
			out.ordered, err = out.repo.ParentsOrderedAll(ctx)
		}
		if err == nil {
			out.graph, out.graphIndex, err = logview.NewGraphRows(out.ordered)
		}
		if err == nil {
			out.commits, err = out.repo.Log(ctx, repo.LogQuery{MaxCount: pageSize, All: true})
		}
		if err == nil {
			out.done = len(out.commits) < pageSize
		}
		out.err = err
		m.publish(&out)
	}()
}

func (m *Model) loadPage(branch string, skip int) {
	if m.repo == nil {
		return
	}
	ctx, repository := m.ctx, m.repo
	if ctx == nil {
		ctx = context.Background()
	}
	generation := m.generation
	m.log.Loading, m.log.Error = true, ""
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		query := repo.LogQuery{MaxCount: pageSize, Skip: skip, All: true}
		if branch != "" && branch != "All branches" {
			query.Branch = branch
		}
		commits, err := repository.Log(ctx, query)
		m.publish(&loadResult{kind: resultPage, generation: generation, commits: commits, done: len(commits) < pageSize, err: err})
	}()
}

func (m *Model) loadDetails(hash model.Hash) {
	if m.repo == nil {
		return
	}
	ctx, generation, repository := m.ctx, m.generation, m.repo
	if ctx == nil {
		ctx = context.Background()
	}
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		files, err := repository.ChangedFiles(ctx, hash)
		m.publish(&loadResult{kind: resultDetails, generation: generation, hash: hash, details: files, err: err})
	}()
}

func (m *Model) changeBranch(branch string) {
	if m.repo == nil {
		return
	}
	m.cancelCurrent()
	generation := m.nextGeneration()
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel = ctx, cancel
	m.log.ResetBranch(branch)
	m.log.SetRefs(m.repo.HEAD, m.repo.Branches, m.repo.Tags)
	repository := m.repo
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		query := repo.LogQuery{MaxCount: pageSize, All: true}
		var ordered []repo.CommitParents
		var err error
		if branch == "All branches" {
			ordered, err = repository.ParentsOrderedAll(ctx)
		} else {
			query.Branch = branch
			ordered, err = repository.ParentsOrdered(ctx, branch)
		}
		var graphRows *logview.GraphRows
		var graphIndex map[model.Hash]int
		if err == nil {
			graphRows, graphIndex, err = logview.NewGraphRows(ordered)
		}
		var commits []model.Commit
		if err == nil {
			commits, err = repository.Log(ctx, query)
		}
		m.publish(&loadResult{kind: resultOpen, generation: generation, path: repository.Path, branch: branch, repo: repository, ordered: ordered, graph: graphRows, graphIndex: graphIndex, commits: commits, done: len(commits) < pageSize, err: err})
	}()
}

func (m *Model) publish(result *loadResult) {
	m.mu.Lock()
	accepted := !m.closed && result.generation == m.generation
	if accepted {
		m.pending = append(m.pending, result)
	}
	m.mu.Unlock()
	if accepted {
		m.invalidator.Invalidate()
	}
}

// applyPending runs only on Viva's window thread, before the repaint it wakes.
func (m *Model) applyPending() {
	m.mu.Lock()
	results := m.pending
	m.pending = nil
	m.mu.Unlock()
	for _, result := range results {
		m.applyResult(result)
	}
}

func (m *Model) applyResult(result *loadResult) {
	if result == nil || result.generation != m.generation {
		return
	}
	if result.err != nil {
		if result.kind == resultOpen && m.repo == nil {
			m.log.Loading = false
			m.err = result.err.Error()
		} else {
			m.log.SetError(result.err)
		}
		return
	}
	switch result.kind {
	case resultOpen:
		m.repo = result.repo
		m.repoConfig, _ = config.LoadRepo(m.configDir, result.repo.Path)
		m.pathEntry = result.repo.Path
		m.branches, m.choices = nil, []string{"All branches"}
		for _, branch := range result.repo.Branches {
			if branch.Ref.Kind() == model.KindLocalBranch {
				m.branches = append(m.branches, branch.Ref.Name())
				m.choices = append(m.choices, branch.Ref.Name())
			}
		}
		m.log.Choices, m.log.Branches = m.choices, m.branches
		if result.branch == "" {
			m.log.Branch = "All branches"
		} else {
			m.log.Branch = result.branch
		}
		m.log.SetRefs(result.repo.HEAD, result.repo.Branches, result.repo.Tags)
		m.log.ResetBranch(m.log.Branch)
		m.log.SetGraph(result.graph, result.graphIndex)
		m.log.AppendPage(result.commits, result.done)
		if len(result.commits) > 0 {
			m.log.Select(result.commits[0].Hash)
			m.loadDetails(result.commits[0].Hash)
		}
		m.err = ""
		if err := m.appConfig.TouchRecent(result.repo.Path, filepath.Base(result.repo.Path), time.Now().UnixMilli()); err == nil {
			_ = config.SaveApp(m.configDir, m.appConfig)
		}
	case resultPage:
		m.log.AppendPage(result.commits, result.done)
	case resultDetails:
		if result.hash.Equal(m.log.Selected()) {
			m.log.Details = result.details
		}
	}
}

func (m *Model) returnToPicker() {
	m.cancelCurrent()
	m.nextGeneration()
	m.repo = nil
	m.log.Loading = false
}
func (m *Model) cancelCurrent() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}
func (m *Model) stopWorkers() {
	m.mu.Lock()
	m.closed = true
	m.pending = nil
	m.mu.Unlock()
	m.cancelCurrent()
	m.workers.Wait()
}

func (m *Model) nextGeneration() uint64 {
	m.mu.Lock()
	m.generation++
	generation := m.generation
	m.mu.Unlock()
	return generation
}
