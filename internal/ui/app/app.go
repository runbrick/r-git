package app

import (
	"context"
	"github.com/runbrick/r-git/internal/i18n"
	"path/filepath"
	"sync"
	"time"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	branchui "github.com/runbrick/r-git/internal/ui/branches"
	commitview "github.com/runbrick/r-git/internal/ui/commit"
	"github.com/runbrick/r-git/internal/ui/controls"
	"github.com/runbrick/r-git/internal/ui/icons"
	logview "github.com/runbrick/r-git/internal/ui/log"
	apptheme "github.com/runbrick/r-git/internal/ui/theme"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const pageSize = 100

type resultKind uint8

const (
	resultOpen resultKind = iota
	resultPage
	resultDetails
	resultWorkflow
	resultCommit
	resultBranch
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
	containing []string
	hash       model.Hash
	done       bool
	err        error
	workflow   *workflowResult
	changes    []repo.ChangeEntry
	message    string
	toast      string
	committed  bool
	pushAfter  bool
	note       string
	snap       repo.Snapshot
	ui         *branchui.UIEvent
}

// Model owns the application shell and coordinates repository work.
type Model struct {
	language string
	logTitle i18n.NameText
	i18n.Locale
	configDir     string
	appConfig     config.AppSettings
	repoConfig    config.RepoSettings
	initial       string
	pathEntry     string
	repo          *repo.Repo
	log           logview.View
	commit        commitview.View
	showCommit    bool
	showBranches  bool
	theme         ui.Theme
	settingsOpen  bool
	projectsOpen  bool
	branchOpen    bool
	branchPopup   branchui.Panel
	branches      branchui.Session
	runner        *command.Runner
	localBranches []string
	choices       []string
	err           string

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
	tools       tools
	gitCancels  []context.CancelFunc
	// seenFrame is closed when the first Page call returns. The headless driver
	// binds the invalidator after that return, so the first open waits for it.
	seenFrame chan struct{}
}

// New loads settings. Repository I/O starts after the window's first paint.
func New(configDir, initialPath string) (*Model, error) {
	settings, err := config.LoadApp(configDir)
	if err != nil {
		return nil, err
	}
	m := &Model{configDir: configDir, appConfig: settings, initial: initialPath, pathEntry: initialPath, seenFrame: make(chan struct{})}
	startupLanguage := i18n.StartupLanguage(settings.Language)
	m.appConfig.Language = settings.Language
	m.SetLanguage(startupLanguage)
	m.appConfig.Language = settings.Language
	m.showCommit = true
	m.theme = apptheme.Dark()
	if settings.Theme == "light" {
		m.theme = apptheme.Light()
	}
	m.log.Branch = "All branches"
	m.log.Choices = []string{"All branches"}
	m.log.Done = initialPath == ""
	m.log.OnBranch = m.changeBranch
	m.log.OnPage = m.loadPage
	m.log.OnSelect = m.loadDetails
	m.log.Toolbar = m.networkToolbar
	m.log.OnRefresh = m.refresh
	m.commit.OnRefresh = m.refreshStatus
	m.initTools()
	m.commit.OnRequest = m.handleCommitRequest
	m.commit.OnShowDiff = func(hash model.Hash, path string) {
		if m.OnShowDiff != nil {
			m.OnShowDiff(hash, path)
		}
	}
	m.OnFetch = func() { m.branches.Handle(branchui.Request{Action: branchui.ActionFetch}, m.startGit, m.refresh) }
	m.OnPull = func() { m.branches.Handle(branchui.Request{Action: branchui.ActionPull}, m.startGit, m.refresh) }
	m.OnPush = func() { m.branches.Handle(branchui.Request{Action: branchui.ActionPush}, m.startGit, m.refresh) }
	m.branchPopup.OnRequest = func(req branchui.Request) {
		m.branchOpen = false
		m.branches.Handle(req, m.startGit, m.refresh)
	}
	return m, nil
}

// Run opens the Viva window.
func Run(configDir, initialPath string) error {
	m, err := New(configDir, initialPath)
	if err != nil {
		return err
	}
	page := ui.NewPage(ui.PageConfig{View: m.Page, Theme: &m.theme, Invalidator: &m.invalidator})
	var canvas gfx.Canvas
	return viva.RunCanvas(page.Config(viva.Config{Title: "r-git", Width: 1400, Height: 900,
		OnWake:   m.applyPending,
		OnClosed: func() { m.stopWorkers() },
	}), &canvas)
}

func (m *Model) config() viva.Config {
	page := ui.NewPage(ui.PageConfig{View: m.Page, Theme: &m.theme, Invalidator: &m.invalidator})
	return page.Config(viva.Config{Title: "r-git", Width: 1400, Height: 900,
		OnWake:   m.applyPending,
		OnClosed: func() { m.stopWorkers() },
	})
}

// Page composes the toolbar and either the repository picker or log view.
func (m *Model) Page(c ui.C) {
	if m.repo == nil && m.initial != "" && !m.startQueued {
		m.startQueued = true
		m.startOpen(m.initial)
	}
	theme := c.Theme()
	ui.Column(c).Fill().Background(theme.Card).Children(func() {
		m.toolbar(c)
		if m.repo == nil {
			m.startPage(c)
		} else {
			ui.Row(c).Fill().Gap(4).Children(func() {
				m.stripe(c)
				if m.showCommit || m.showBranches {
					ui.Column(c).Width(320).FillHeight().Background(theme.Background).Radius(8).Children(func() {
						if m.showCommit {
							m.commit.Page(c)
						} else {
							m.branches.Panel.Page(c)
							if controls.IconButton(c, icons.Pair(c, icons.Stash, icons.Stash_dark), m.Text().OpenStash).Clicked() {
								c.After(func() { m.branches.Handle(branchui.Request{Action: branchui.ActionStash}, m.startGit, m.refresh) })
							}
						}
					})
				}
				ui.Column(c).Fill().Background(theme.Background).Radius(8).Children(func() {
					m.editorTabs(c)
					if !m.toolsPage(c) {
						m.log.Page(c)
					}
				})
			})
			ui.Row(c).FillWidth().Height(24).PadX(16).Gap(12).Items(ui.Center).Children(func() {
				ui.Text(c, m.repo.Path).FontSize(12).Muted()
				ui.Spacer(c)
				ui.Text(c, m.branchName()).FontSize(12).Muted()
			})
		}
		m.branches.Overlays(c)
		c.After(func() { m.branches.AfterPaint(m.startGit, m.refresh) })
	})
	m.settings(c)
	m.noteFrame()
}

// noteFrame lets an open that the first paint armed start. The driver binds
// the invalidator after this call returns.
func (m *Model) noteFrame() {
	select {
	case <-m.seenFrame:
	default:
		close(m.seenFrame)
	}
}

func (m *Model) toolbar(c ui.C) {
	t := c.Theme()
	ui.Row(c).FillWidth().Height(44).PadX(12).Gap(8).Items(ui.Center).Background(t.Card).Children(func() {
		ui.Text(c, "r-git").Bold().Color(t.Primary).Width(40)
		name := m.Text().OpenRepository
		if m.repo != nil {
			name = filepath.Base(m.repo.Path)
		}
		project := ui.ButtonBase(c, name).Height(28).PadX(8).Gap(8).Radius(4).Tooltip(m.Text().RecentProjects).Hover(ui.Look{}.Background(t.Hover)).Focus(ui.Look{}.Ring(2, t.Ring)).Children(func() {
			ui.Icon(c, icons.Pair(c, icons.Folder, icons.Folder_dark)).Size(16, 16)
			ui.Text(c, name).FontSize(13)
			ui.Icon(c, icons.Pair(c, icons.Down, icons.Down_dark)).Size(16, 16)
		})
		if project.Clicked() {
			m.projectsOpen = !m.projectsOpen
		}
		ui.Menu(c, project, &m.projectsOpen, func(menu *ui.MenuItems) {
			for _, recent := range m.appConfig.Recent {
				item := recent
				if menu.Item(item.DisplayName).Clicked() {
					c.After(func() { m.startOpen(item.Path) })
				}
			}
			if menu.Item(m.Text().OpenRepository).Clicked() {
				c.After(m.returnToPicker)
			}
		})
		if m.repo != nil {
			branch := ui.ButtonBase(c, m.branchName()).Height(28).PadX(8).Gap(8).Radius(4).Tooltip(m.Text().Branches).Hover(ui.Look{}.Background(t.Hover)).Focus(ui.Look{}.Ring(2, t.Ring)).Children(func() {
				ui.Icon(c, icons.Pair(c, icons.Branches, icons.Branches_dark)).Size(16, 16)
				ui.Text(c, m.branchName()).FontSize(13)
				ui.Icon(c, icons.Pair(c, icons.Down, icons.Down_dark)).Size(16, 16)
			})
			if branch.Clicked() {
				c.After(func() {
					m.branchOpen = !m.branchOpen
					if m.branchOpen {
						// Each visible tree owns its selection and disclosure state.
						m.branchPopup.Nodes = append(m.branchPopup.Nodes[:0], m.branches.Panel.Nodes...)
						m.branchPopup.Selected = m.branches.Panel.Selected
					}
				})
			}
			ui.Popover(c, branch, &m.branchOpen, func() {
				ui.Column(c).Width(304).Height(440).Gap(4).Children(func() {
					m.networkToolbar(c)
					m.branchPopup.Page(c)
				})
			})
		}
		ui.Spacer(c)
		if controls.IconButton(c, icons.Pair(c, icons.Refresh, icons.Refresh_dark), m.Text().Refresh).Disabled(m.repo == nil || m.tools.busy).Clicked() {
			c.After(m.refresh)
		}
		if controls.IconButton(c, icons.Pair(c, icons.Settings, icons.Settings_dark), m.Text().Settings).Clicked() {
			m.settingsOpen = true
		}
	})
}

func (m *Model) branchName() string {
	if m.repo != nil && m.repo.CurrentBranch.Name() != "" {
		return m.repo.CurrentBranch.Name()
	}
	return "HEAD"
}

func (m *Model) stripe(c ui.C) {
	ui.Column(c).Width(36).FillHeight().Gap(8).PadY(4).Items(ui.Center).Children(func() {
		commit := controls.IconButton(c, icons.Pair(c, icons.Commit, icons.Commit_dark), m.Text().Commit)
		if m.showCommit {
			commit.Background(c.Theme().Hover)
		}
		if commit.Clicked() {
			c.After(func() {
				m.showCommit = !m.showCommit
				m.showBranches = false
				if m.showCommit {
					m.refreshStatus()
				}
				if m.OnCommit != nil {
					m.OnCommit()
				}
			})
		}
		branches := controls.IconButton(c, icons.Pair(c, icons.Branches, icons.Branches_dark), m.Text().Branches)
		if m.showBranches {
			branches.Background(c.Theme().Hover)
		}
		if branches.Clicked() {
			m.showBranches = !m.showBranches
			m.showCommit = false
			m.branchOpen = false
		}
	})
}

func (m *Model) networkToolbar(c ui.C) {
	ui.Row(c).Gap(4).Items(ui.Center).Children(func() {
		if controls.IconButton(c, icons.Pair(c, icons.Fetch, icons.Fetch_dark), m.Text().Fetch).Disabled(m.OnFetch == nil).Clicked() {
			c.After(m.OnFetch)
		}
		if controls.IconButton(c, icons.Pair(c, icons.Pull, icons.Pull_dark), m.Text().Pull).Disabled(m.OnPull == nil).Clicked() {
			c.After(m.OnPull)
		}
		if controls.IconButton(c, icons.Pair(c, icons.Push, icons.Push_dark), m.Text().Push).Disabled(m.OnPush == nil).Clicked() {
			c.After(m.OnPush)
		}
	})
}

func (m *Model) editorTabs(c ui.C) {
	t := c.Theme()
	ui.Row(c).FillWidth().Height(36).PadX(8).Gap(4).Items(ui.Center).FocusGroup(ui.FocusHorizontal).Border(1, t.Border).Children(func() {
		branch := m.branchName()
		if m.log.Branch != "All branches" {
			branch = m.log.Branch
		}
		if m.editorTab(c, "", m.logTitle.Get(m.Text(), branch, m.Text().LogTitle), icons.Pair(c, icons.Branches, icons.Branches_dark)).Clicked() {
			m.tools.rebase.Open = false
		}
		if m.editorTab(c, "merge", m.Text().Conflicts, icons.Pair(c, icons.Merge, icons.Merge_dark)).Clicked() {
			c.After(m.showConflicts)
		}
		if m.editorTab(c, "rebase", m.Text().InteractiveRebase, icons.Pair(c, icons.Rebase, icons.Rebase_dark)).Clicked() {
			m.tools.rebase.Open = true
		}

		if m.tools.screen == "diff" {
			ui.Text(c, m.Text().Diff).PadX(8).Color(t.Primary)
		}
		ui.Spacer(c)
	})
}

func (m *Model) editorTab(c ui.C, screen, label string, icon gfx.Icon) ui.Box {
	t := c.Theme()
	tab := ui.TabBase(c, &m.tools.screen, screen, label).Height(28).PadX(8).Gap(8).Radius(4).
		Disabled(m.tools.busy && screen != "").
		Hover(ui.Look{}.Background(t.Hover)).Focus(ui.Look{}.Ring(2, t.Ring))
	if m.tools.screen == screen {
		tab.Background(t.Selection)
	}
	return tab.Children(func() {
		ui.Icon(c, icon).Size(16, 16)
		ui.Text(c, label).FontSize(13)
	})
}

var themeChoices = []string{"dark", "light"}
var languageChoices = []string{"zh-CN", "en"}

// SetLanguage applies a choice on the window thread, including loaded caches.
func (m *Model) SetLanguage(language string) {
	m.language = language
	m.appConfig.Language = language
	m.UseMessages(i18n.ForLanguage(language))
	m.log.SetMessages(m.Text())
	m.commit.SetMessages(m.Text())
	m.branches.SetMessages(m.Text())
	m.branchPopup.UseMessages(m.Text())
	m.tools.diff.SetMessages(m.Text())
	m.tools.merge.SetMessages(m.Text())
	m.tools.rebase.UseMessages(m.Text())
}

func (m *Model) settings(c ui.C) {
	ui.ModalWith(c, &m.settingsOpen, m.Text().Settings, ui.ModalSpec{NoClose: m.Text() != &i18n.En}, func() {
		ui.Column(c).Width(320).Gap(12).Children(func() {
			language := m.language
			ui.Select(c, &language, languageChoices, i18n.LanguageChoice).Label(m.Text().Language)
			if language != m.language {
				c.After(func() {
					m.SetLanguage(language)
					if err := config.SaveApp(m.configDir, m.appConfig); err != nil {
						m.err = m.Text().Error(err)
					}
				})
			}
			ui.Text(c, m.Text().Appearance).Bold()
			before := m.appConfig.Theme
			ui.Select(c, &m.appConfig.Theme, themeChoices, m.Text().ThemeChoice).Label(m.Text().Theme)
			if m.appConfig.Theme != before {
				if m.appConfig.Theme == "light" {
					m.theme = apptheme.Light()
				} else {
					m.theme = apptheme.Dark()
				}
				c.After(func() {
					if err := config.SaveApp(m.configDir, m.appConfig); err != nil {
						m.err = m.Text().Error(err)
					}
				})
			}
			if m.err != "" {
				ui.Text(c, m.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, m.Text().OK).Clicked() {
			m.settingsOpen = false
		}
	})
}

func (m *Model) startPage(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(16).Children(func() {
		ui.Text(c, m.Text().OpenRepository).FontSize(20).Bold()
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			field := ui.TextField(c, &m.pathEntry, m.Text().RepositoryPath, m.Text().PathToAGitWorkingTree).Grow(1)
			if field.Submitted() || ui.Button(c, m.Text().Open).Clicked() {
				path := m.pathEntry
				c.After(func() { m.startOpen(path) })
			}
		})
		if m.err != "" {
			ui.Text(c, m.err).Color(c.Theme().Danger)
		}
		if len(m.appConfig.Recent) > 0 {
			ui.Text(c, m.Text().RecentRepositories).FontSize(13).Muted()
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, recent := range m.appConfig.Recent {
					item := recent
					ui.Row(c.Key(item.Path)).FillWidth().Pad(8).Gap(12).Items(ui.Center).Background(c.Theme().Card).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() { ui.Text(c, item.DisplayName).Bold(); ui.Text(c, item.Path).FontSize(12).Muted() })
						if ui.Button(c, m.Text().Open).Clicked() {
							c.After(func() { m.startOpen(item.Path) })
						}
					})
				}
			})
		}
		if m.log.Loading {
			ui.Text(c, m.Text().LoadingRepositoryEllipsis).FontSize(12).Muted()
			// The spinner asks for the next frame. Headless WaitFor delivers that
			// frame through OnWake, which applies a result a cross-thread
			// Invalidate did not deliver on Linux.
			ui.Spinner(c)
		}
	})
}

func (m *Model) startOpen(path string) {
	m.prepareOpen(path)
}

// prepareOpen loads path. The first call waits until the first paint returns,
// which is when the driver binds the invalidator.
func (m *Model) prepareOpen(path string) {
	m.resetTools()
	path = filepath.Clean(path)
	previous := m.cancel
	generation := m.nextGeneration()
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel, m.log.Loading = ctx, cancel, true
	if previous != nil {
		previous()
	}
	m.log.Error, m.err = "", ""
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		select {
		case <-ctx.Done():
			return
		case <-m.seenFrame:
		}
		out := m.loadOpen(ctx, path, generation)
		m.publish(&out)
	}()
}

func (m *Model) loadOpen(ctx context.Context, path string, generation uint64) loadResult {
	out := loadResult{kind: resultOpen, generation: generation, path: path}
	runner, err := command.New()
	if err == nil {
		out.repo, err = repo.OpenWithExecutor(ctx, path, runner)
	}
	if err == nil {
		out.ordered, err = out.repo.ParentsOrderedAll(ctx)
	}
	if err == nil {
		out.graph, out.graphIndex, err = logview.NewGraphRows(out.ordered, graphReferences(out.repo)...)
	}
	if err == nil {
		out.commits, err = out.repo.Log(ctx, repo.LogQuery{MaxCount: pageSize, All: true})
	}
	if err == nil {
		out.done = len(out.commits) < pageSize
	}
	out.err = err
	if err == nil {
		out.snap, err = out.repo.Snapshot(ctx)
		out.err = err
	}
	return out
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
		var containing []string
		if err == nil {
			containing, err = repository.ContainingBranches(ctx, hash)
		}
		m.publish(&loadResult{kind: resultDetails, generation: generation, hash: hash, details: files, containing: containing, err: err})
	}()
}

func (m *Model) changeBranch(branch string) {
	if m.repo == nil {
		return
	}
	m.resetTools()
	m.cancelCurrent()
	generation := m.nextGeneration()
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel = ctx, cancel
	m.log.ResetBranch(branch)
	m.log.CurrentBranch = m.repo.CurrentBranch.Name()
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
			graphRows, graphIndex, err = logview.NewGraphRows(ordered, graphReferences(repository)...)
		}
		var commits []model.Commit
		if err == nil {
			commits, err = repository.Log(ctx, query)
		}
		var snap repo.Snapshot
		if err == nil {
			snap, err = repository.Snapshot(ctx)
		}
		m.publish(&loadResult{kind: resultOpen, generation: generation, path: repository.Path, branch: branch, repo: repository, ordered: ordered, graph: graphRows, graphIndex: graphIndex, commits: commits, done: len(commits) < pageSize, snap: snap, err: err})
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
	if result.kind == resultWorkflow {
		m.applyWorkflow(result.workflow)
		return
	}
	if result.kind == resultCommit {
		m.applyCommit(result)
		return
	}
	if result.kind == resultBranch {
		m.applyBranch(result)
		return
	}
	if result.err != nil {
		if result.kind == resultOpen && m.repo == nil {
			m.log.Loading = false
			m.err = m.Text().Error(result.err)
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
		m.localBranches, m.choices = nil, []string{"All branches"}
		for _, branch := range result.repo.Branches {
			if branch.Ref.Kind() == model.KindLocalBranch {
				m.localBranches = append(m.localBranches, branch.Ref.Name())
				m.choices = append(m.choices, branch.Ref.Name())
			}
		}
		m.log.Choices, m.log.Branches = m.choices, m.localBranches
		if result.branch == "" {
			m.log.Branch = "All branches"
		} else {
			m.log.Branch = result.branch
		}
		m.log.CurrentBranch = result.repo.CurrentBranch.Name()
		m.log.SetRefs(result.repo.HEAD, result.repo.Branches, result.repo.Tags)
		m.log.ResetBranch(m.log.Branch)
		m.log.SetGraph(result.graph, result.graphIndex)
		m.log.AppendPage(result.commits, result.done)
		if len(result.commits) > 0 {
			m.log.Select(result.commits[0].Hash)
			m.loadDetails(result.commits[0].Hash)
		}
		m.err = ""
		m.bindBranches(result.snap)
		m.commit.SetChanges(repo.GroupChanges(result.snap.Changes))
		if m.showCommit {
			m.refreshStatus()
		}
		if err := m.appConfig.TouchRecent(result.repo.Path, filepath.Base(result.repo.Path), time.Now().UnixMilli()); err == nil {
			_ = config.SaveApp(m.configDir, m.appConfig)
		}
	case resultPage:
		m.log.AppendPage(result.commits, result.done)
	case resultDetails:
		if result.hash.Equal(m.log.Selected()) {
			m.log.SetDetails(result.details)
			m.log.SetContaining(result.containing)
		}
	}
}

func (m *Model) applyCommit(result *loadResult) {
	m.commit.Busy = false
	m.commit.SetChanges(result.changes)
	if result.message != "" {
		m.commit.SetLastMessage(result.message)
	}
	if result.err != nil {
		m.commit.Error = commitview.UserMessageFor(m.Text(), result.err)
		m.commit.ShowToast(m.Text().CommitFailed, m.commit.Error, ui.ToastError)
		return
	}
	if result.committed {
		m.commit.ResetAfterCommit()
		m.commit.ShowToast(m.Text().Committed, result.toast, ui.ToastSuccess)
		if result.pushAfter && m.OnPush != nil {
			m.OnPush()
		}
		m.refreshLog()
	} else if result.toast != "" {
		m.commit.ShowToast(m.Text().Changes, result.toast, ui.ToastSuccess)
	}
}

func (m *Model) applyBranch(result *loadResult) {
	if result.ui != nil {
		m.branches.ApplyUI(*result.ui)
		if result.ui.Refresh {
			m.refresh()
		}
		return
	}
	if result.err != nil {
		m.branches.Panel.Error = m.Text().Error(result.err)
		return
	}
	m.bindBranches(result.snap)
}

func (m *Model) bindBranches(snap repo.Snapshot) {
	if m.runner == nil {
		runner, err := command.New()
		if err != nil {
			m.branches.Panel.Error = m.Text().Error(err)
			return
		}
		m.runner = runner
	}
	m.branches.SetPublisher(m.publishUI)
	m.branches.SetRunner(m.startGit, m.refresh)
	ahead := map[string][2]int{}
	if name := snap.CurrentBranch.Name(); name != "" {
		ahead[name] = [2]int{snap.Ahead, snap.Behind}
	}
	m.branches.Bind(m.repo, m.runner, m.repoConfig, &m.invalidator)
	m.branches.SetSnapshot(snap, ahead)
}

func (m *Model) publishUI(event branchui.UIEvent) {
	copied := event
	m.publish(&loadResult{kind: resultBranch, generation: m.generation, ui: &copied})
}

func (m *Model) startGit(work func(context.Context)) {
	if m.repo == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.gitCancels = append(m.gitCancels, cancel)
	m.mu.Unlock()
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		defer cancel()
		work(ctx)
	}()
}

func (m *Model) refresh() {
	if m.repo == nil {
		return
	}
	m.startOpen(m.repo.Path)
}

func (m *Model) returnToPicker() {
	m.resetTools()
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
	cancels := append([]context.CancelFunc(nil), m.gitCancels...)
	m.mu.Unlock()
	m.cancelCurrent()
	for _, cancel := range cancels {
		cancel()
	}
	m.workers.Wait()
}

func (m *Model) nextGeneration() uint64 {
	m.mu.Lock()
	m.generation++
	generation := m.generation
	m.mu.Unlock()
	return generation
}

func graphReferences(repository *repo.Repo) []repo.Reference {
	refs := make([]repo.Reference, 0, len(repository.Branches)+len(repository.Tags)+1)
	refs = append(refs, repository.Branches...)
	refs = append(refs, repository.Tags...)
	if !repository.HEAD.IsZero() {
		refs = append(refs, repo.Reference{Ref: model.HeadRef(), Hash: repository.HEAD})
	}
	return refs
}
