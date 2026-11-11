package log

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/runbrick/r-git/internal/graph/printer"
	"github.com/runbrick/r-git/internal/ui/controls"
	"github.com/runbrick/r-git/internal/ui/icons"
	apptheme "github.com/runbrick/r-git/internal/ui/theme"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// View owns the virtualized history table, filters, graph and selection.
type View struct {
	Commits          []model.Commit
	Graph            *GraphRows
	GraphIndex       map[model.Hash]int
	Refs             map[model.Hash][]string
	Head             model.Hash
	Details          []model.FileChange
	Filter           string
	Regex, MatchCase bool
	User             string
	Date             string
	FilterError      string
	HideDetails      bool
	ShowHash         bool
	CurrentBranch    string
	OnRefresh        func()
	optionsOpen      bool
	userOpen         bool
	refKinds         map[string]model.Kind
	branchLine       string
	Branch           string
	Branches         []string
	Choices          []string
	Loading          bool
	Done             bool
	Error            string
	OnBranch         func(string)
	OnPage           func(string, int)
	OnSelect         func(model.Hash)
	OnGraphBuilt     func()
	OnShowDiff       func(model.Hash, string)
	Toolbar          func(ui.C)
	files            ui.TreeState
	fileNodes        []fileNode
	fileExpanded     map[string]bool
	fileC            ui.C
	detailSource     []model.FileChange
	parentLabel      string

	list             ui.ListState
	columns          commitColumns
	visible          []int
	searchText       []string
	dates            []string
	syncSelection    bool
	normalizedFilter string
	selected         model.Hash
}

// SetRefs caches ref labels once per repository snapshot.
func (v *View) SetRefs(head model.Hash, branches, tags []repo.Reference) {
	if v.Date == "" {
		v.Date = "Any date"
	}
	v.Head = head
	v.refKinds = make(map[string]model.Kind)
	v.Refs = make(map[model.Hash][]string, len(branches)+len(tags)+1)
	for _, ref := range branches {
		v.refKinds[ref.Ref.Name()] = ref.Ref.Kind()
		v.Refs[ref.Hash] = append(v.Refs[ref.Hash], ref.Ref.Name())
	}
	for _, ref := range tags {
		v.refKinds[ref.Ref.Name()] = model.KindTag
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
	if graphRows != nil {
		v.columns.graphWidth = graphRows.width
	}
	if v.OnGraphBuilt != nil {
		v.OnGraphBuilt()
	}
	v.rebuildVisible()
}

// AppendPage adds one metadata page and refreshes the filter index.
func (v *View) AppendPage(commits []model.Commit, done bool) {
	for i := range commits {
		v.dates = append(v.dates, shortDate(commits[i].Author.When))
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
	v.cacheParents()
	v.syncSelection = true
}

// ResetBranch clears branch-scoped commit state while retaining reusable list storage.
func (v *View) ResetBranch(branch string) {
	v.Branch, v.Commits, v.Details = branch, nil, nil
	v.detailSource, v.fileNodes = nil, nil
	v.searchText = v.searchText[:0]
	v.dates = v.dates[:0]
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
	v.normalizedFilter = strings.TrimSpace(v.Filter)
	v.FilterError = ""
	var pattern *regexp.Regexp
	if v.Regex && v.normalizedFilter != "" {
		query := v.normalizedFilter
		if !v.MatchCase {
			query = "(?i)" + query
		}
		var err error
		pattern, err = regexp.Compile(query)
		if err != nil {
			v.FilterError = "Invalid regular expression"
		}
	}
	needle := v.normalizedFilter
	if !v.MatchCase {
		needle = strings.ToLower(needle)
	}
	now := time.Now()
	var since, until time.Time
	switch v.Date {
	case "Today":
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "Last 7 days":
		since = now.AddDate(0, 0, -7)
	case "Last 30 days":
		since = now.AddDate(0, 0, -30)
	case "Before today":
		until = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	v.visible = v.visible[:0]
	for i, commit := range v.Commits {
		if v.FilterError != "" {
			continue
		}
		if needle != "" {
			text := v.searchText[i]
			if v.MatchCase {
				text = commit.Subject + " " + commit.Hash.String()
			}
			if pattern != nil {
				if !pattern.MatchString(text) {
					continue
				}
			} else if !strings.Contains(text, needle) {
				continue
			}
		}
		if v.User != "" && !strings.Contains(strings.ToLower(commit.Author.Name+" "+commit.Author.Email), strings.ToLower(v.User)) {
			continue
		}
		when := commit.Committer.When
		if !since.IsZero() && when.Before(since) || !until.IsZero() && !when.Before(until) {
			continue
		}
		v.visible = append(v.visible, i)
	}
	// Apply the hash selection after List knows the new filtered row count.
	v.syncSelection = true
}

var dateFilters = []string{"Any date", "Today", "Last 7 days", "Last 30 days", "Before today"}

func (v *View) toolbar(c ui.C) {
	t := c.Theme()
	ui.Row(c).FillWidth().PadX(8).PadY(4).Gap(4).Items(ui.Center).Children(func() {
		oldFilter, oldBranch, oldUser, oldDate := v.Filter, v.Branch, v.User, v.Date
		ui.TextField(c, &v.Filter, "Filter", "Text or hash").Width(172).MinWidth(100).Grow(1)
		toggle := func(label, text string, on *bool) {
			button := controls.TextButton(c, label, text).Width(40).Shrink(0).Tooltip(label)
			if *on {
				button.Background(t.Selection)
			}
			if button.Clicked() {
				*on = !*on
				v.rebuildVisible()
			}
		}
		toggle("Regex", ".*", &v.Regex)
		toggle("Match case", "Cc", &v.MatchCase)
		ui.Select(c, &v.Branch, v.Choices, nil).Label("Branch").Width(128).Shrink(0)
		user := ui.ButtonBase(c, "User").Width(80).Height(28).Shrink(0).PadX(8).
			Radius(t.Metrics.RadiusControl).Border(1, t.Input).Tooltip("Filter by author").
			Hover(ui.Look{}.Background(t.Hover)).Pressed(ui.Look{}.Background(t.Muted)).Focus(ui.Look{}.Ring(2, t.Ring)).
			Children(func() {
				ui.Text(c, "User").FontSize(13).Grow(1)
				ui.Icon(c, icons.Pair(c, icons.Down, icons.Down_dark)).Size(16, 16)
			})
		if v.User != "" {
			user.Background(t.Selection)
		}
		if user.Clicked() {
			v.userOpen = !v.userOpen
		}
		ui.Popover(c, user, &v.userOpen, func() { ui.TextField(c, &v.User, "User filter", "Author contains").Width(200) })
		ui.Select(c, &v.Date, dateFilters, nil).Label("Date").Width(128).Shrink(0)
		if v.Toolbar != nil {
			v.Toolbar(c)
		}
		if controls.IconButton(c, icons.Pair(c, icons.Refresh, icons.Refresh_dark), "Refresh log").Disabled(v.OnRefresh == nil).Clicked() {
			c.After(v.OnRefresh)
		}
		details := controls.IconButton(c, icons.Pair(c, icons.Show, icons.Show_dark), "Show details")
		if !v.HideDetails {
			details.Background(t.Hover)
		}
		if details.Clicked() {
			v.HideDetails = !v.HideDetails
		}
		options := controls.IconButton(c, icons.Pair(c, icons.More, icons.More_dark), "Log options")
		if options.Clicked() {
			v.optionsOpen = !v.optionsOpen
		}
		ui.Menu(c, options, &v.optionsOpen, func(menu *ui.MenuItems) {
			if menu.Item("Show hash").Checked(v.ShowHash).Clicked() {
				v.ShowHash = !v.ShowHash
			}
		})
		if oldFilter != v.Filter || oldUser != v.User || oldDate != v.Date {
			v.rebuildVisible()
		}
		if oldBranch != v.Branch && v.OnBranch != nil {
			branch := v.Branch
			c.After(func() { v.OnBranch(branch) })
		}
	})
	if v.FilterError != "" {
		ui.Text(c, v.FilterError).FontSize(12).Color(t.Danger)
	}
}

// Page declares the log and details panes.
func (v *View) Page(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Children(func() {
		// Filters and actions belong to the whole log tool window, including
		// details. Sharing its width keeps controls usable at 1200px as well.
		v.toolbar(c)
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Grow(1).Fill().Children(func() {
				v.columns.showHash = v.ShowHash
				if v.list.Label == nil {
					v.list.Label = v.rowLabel
				}
				ui.CustomLayout(c, &v.columns).FillWidth().PadX(12).PadY(4).Background(theme.Card).Children(func() {
					ui.Slot(c)
					ui.Text(c, "Subject").FontSize(11).Muted().Ellipsis()
					ui.Text(c, "Author").FontSize(11).Muted().Ellipsis()
					ui.Text(c, "Date").FontSize(11).Muted().Ellipsis()
					if v.ShowHash {
						ui.Text(c, "Hash").FontSize(11).Muted().Ellipsis()
					}
				})
				ui.List(c, &v.list, len(v.visible), func(row ui.ListRow) {
					index := v.visible[row.Index]
					commit := v.Commits[index]
					selected := row.Selected()
					if v.syncSelection {
						selected = commit.Hash.Equal(v.selected)
					}
					fg := theme.Foreground
					if len(commit.Parents) >= 2 && !selected {
						fg = apptheme.Log(theme).Unmatched
					}
					bg := theme.Background
					if selected {
						bg = theme.Selection
					}
					row.Element.FillWidth().Height(26).Background(bg).Children(func() {
						ui.CustomLayout(row.C, &v.columns).Fill().PadX(12).Children(func() {
							ui.Slot(row.C).Height(26).Clip().Draw(func(d *ui.DrawContext) { v.drawGraph(d, v.GraphIndex[commit.Hash]) })
							ui.Row(row.C).Clip().Gap(6).Items(ui.Center).Children(func() {
								if len(v.Refs[commit.Hash]) > 0 {
									ui.Row(row.C).MaxWidthPercent(45).Clip().Shrink(1).Gap(4).Items(ui.Center).Children(func() {
										for _, label := range v.Refs[commit.Hash] {
											v.refLabel(row.C, label)
										}
									})
								}
								ui.Text(row.C, commit.Subject).Grow(1).FontSize(13).Color(fg).Ellipsis()
							})
							ui.Text(row.C, commit.Author.Name).FontSize(12).Color(fg).Ellipsis()
							ui.Text(row.C, v.dates[index]).FontSize(12).Color(fg).Ellipsis()
							if v.ShowHash {
								ui.Text(row.C, commit.Hash.Short()).FontSize(11).Color(fg).Mono().Ellipsis()
							}
						})
					})
				}).Grow(1)
				if v.syncSelection {
					v.list.Select(-1)
					for row, index := range v.visible {
						if v.Commits[index].Hash.Equal(v.selected) {
							v.list.Select(row)
							break
						}
					}
					v.syncSelection = false
				}
				if selected := v.list.Selected(); selected >= 0 && selected < len(v.visible) {
					hash := v.Commits[v.visible[selected]].Hash
					if !hash.Equal(v.selected) {
						v.selected = hash
						v.cacheParents()
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
				ui.Row(c).FillWidth().PadX(12).PadY(4).Gap(8).Items(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("%d commits", len(v.Commits))).FontSize(11).Muted()
					if v.Loading {
						ui.Text(c, "Loading...").FontSize(11).Muted()
					}
					if v.Error != "" {
						ui.Text(c, v.Error).FontSize(12).Color(theme.Danger)
					}
				})
			})
			if !v.HideDetails {
				ui.Divider(c)
				v.detailPane(c)
			}
		})
	})
}

func (v *View) rowLabel(row int) string {
	if row < 0 || row >= len(v.visible) {
		return ""
	}
	return v.Commits[v.visible[row]].Subject
}

// SetDetails installs changed files and builds the tree only when data arrives.
func (v *View) SetDetails(files []model.FileChange) {
	v.Details = files
	v.detailSource = files
	v.fileExpanded = make(map[string]bool)
	v.rebuildFiles()
}

type fileNode struct {
	name, path, label string
	countLabel        string
	depth             int
	folder            bool
	expanded          bool
}

func (v *View) rebuildFiles() {
	v.fileNodes = v.fileNodes[:0]
	files := append([]model.FileChange(nil), v.Details...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	v.appendFiles(files, "", 0)
}

func (v *View) appendFiles(files []model.FileChange, prefix string, depth int) {
	groups := make(map[string][]model.FileChange)
	var dirs []string
	var leaves []model.FileChange
	for _, file := range files {
		rel := strings.TrimPrefix(file.Path, prefix)
		dir, _, nested := strings.Cut(rel, "/")
		if !nested {
			leaves = append(leaves, file)
			continue
		}
		if _, ok := groups[dir]; !ok {
			dirs = append(dirs, dir)
		}
		groups[dir] = append(groups[dir], file)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		full := prefix + dir + "/"
		expanded, known := v.fileExpanded[full]
		if !known {
			expanded = true
		}
		v.fileNodes = append(v.fileNodes, fileNode{name: dir, path: full, label: dir, depth: depth, countLabel: fileCount(len(groups[dir])), folder: true, expanded: expanded})
		if expanded {
			v.appendFiles(groups[dir], full, depth+1)
		}
	}
	for _, file := range leaves {
		label := file.Status.String() + "  " + file.Path
		if file.OrigPath != "" {
			label += "  <- " + file.OrigPath
		}
		v.fileNodes = append(v.fileNodes, fileNode{name: path.Base(file.Path), path: file.Path, label: label, depth: depth})
	}
}

func (v *View) fileNodeAt(i int) ui.TreeNode {
	n := v.fileNodes[i]
	return ui.TreeNode{Depth: n.depth, Branch: n.folder, Expanded: n.expanded}
}
func (v *View) fileLabel(i int) string { return v.fileNodes[i].label }
func (v *View) fileContent(i int) {
	n, c := v.fileNodes[i], v.fileC
	if n.folder {
		ui.Icon(c, icons.Pair(c, icons.Folder, icons.Folder_dark)).Size(16, 16)
		ui.Text(c, n.name).FontSize(13)
		ui.Text(c, n.countLabel).FontSize(12).Muted()
	} else {
		ui.Icon(c, icons.File(c, n.path)).Size(16, 16)
		ui.Text(c, n.name).FontSize(13)
	}
}

func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func (v *View) cacheParents() {
	v.parentLabel = ""
	v.branchLine = ""

	if commit, ok := v.selectedCommit(); ok {
		for i, parent := range commit.Parents {
			if i > 0 {
				v.parentLabel += "  "
			}
			v.parentLabel += parent.Short()
		}
	}
}

func (v *View) detailPane(c ui.C) {
	theme := c.Theme()
	// Retain compatibility with callers assigning Details directly. The source
	// slice changes on publication; expanding a folder also rebuilds explicitly.
	if len(v.detailSource) != len(v.Details) || (len(v.Details) > 0 && &v.detailSource[0] != &v.Details[0]) {
		v.SetDetails(v.Details)
	}
	if v.files.Node == nil {
		v.files.Node, v.files.RowHeight = v.fileNodeAt, 22
	}
	v.fileC = c
	ui.Column(c).Width(300).FillHeight().Background(theme.Background).Children(func() {
		ui.Row(c).FillWidth().Height(32).PadX(12).Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, "Changed files").FontSize(12).Muted()
			ui.Text(c, fileCount(len(v.Details))).FontSize(12).Muted()
		})
		ui.Tree(c, &v.files, len(v.fileNodes), v.fileLabel, v.fileContent).Grow(1)
		if i, ok := v.files.Toggled(); ok && i < len(v.fileNodes) {
			n := v.fileNodes[i]
			v.fileExpanded[n.path] = !n.expanded
			v.rebuildFiles()
		}
		if i, ok := v.files.Chosen(); ok && i < len(v.fileNodes) && !v.fileNodes[i].folder && v.OnShowDiff != nil {
			hash, file := v.selected, v.fileNodes[i].path
			c.After(func() { v.OnShowDiff(hash, file) })
		}
		ui.Divider(c)
		ui.Scroll(c).FillWidth().Height(244).Pad(12).Gap(4).Children(func() {
			commit, ok := v.selectedCommit()
			if !ok {
				ui.Text(c, "Select a commit").Muted()
				return
			}
			ui.Text(c, commit.Subject).FontSize(13).Bold()
			if body := strings.TrimSpace(strings.TrimPrefix(commit.Message, commit.Subject)); body != "" {
				ui.Text(c, body).FontSize(12)
			}
			ui.Text(c, commit.Hash.Short()).FontSize(12).Mono()
			ui.Text(c, "Author: "+commit.Author.Name).FontSize(12)
			ui.Text(c, "<"+commit.Author.Email+">").FontSize(12).Color(theme.Primary)
			ui.Text(c, "authored on "+shortDate(commit.Author.When)).FontSize(12).Muted()
			ui.Text(c, "Committer: "+commit.Committer.Name).FontSize(12)
			ui.Text(c, "committed on "+shortDate(commit.Committer.When)).FontSize(12).Muted()
			ui.Text(c, v.branchLine).FontSize(12)
			if v.parentLabel != "" {
				ui.Text(c, "Parents: "+v.parentLabel).FontSize(12).Mono().Muted()
			}
		})
	})
}

func (v *View) refLabel(c ui.C, label string) {
	t := c.Theme()
	colors := apptheme.Log(t)
	color := colors.Local
	if label == "HEAD" {
		color = colors.Head
	}
	switch v.refKinds[label] {
	case model.KindRemoteBranch:
		color = colors.Remote
	case model.KindTag:
		color = colors.Tag
	}
	ui.Row(c).Shrink(0).Gap(3).PadX(3).Radius(3).Items(ui.Center).Children(func() {
		ui.Slot(c).Size(8, 12).Draw(func(d *ui.DrawContext) {
			p := d.Canvas.PathBuilder()
			p.MoveTo(0, 0)
			p.LineTo(5, 0)
			p.LineTo(8, 6)
			p.LineTo(5, 12)
			p.LineTo(0, 12)
			p.Close()
			d.Canvas.DrawPath(p.Path(8, 12), d.Rect, gfx.PathStyle{Fill: color})
		})
		text := ui.Text(c, label).FontSize(12)
		if label == v.CurrentBranch || label == "HEAD" {
			text.Bold().Background(t.Selection).Radius(3)
		}
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
	dark := d.Theme.Background.Red < 0.5
	height := d.Rect.Height
	middle := height / 2
	line := func(x1, y1, x2, y2 float32, color ui.Color) {
		p := d.Canvas.PathBuilder()
		p.MoveTo(x1, y1)
		p.LineTo(x2, y2)
		d.Canvas.DrawPath(p.Path(d.Rect.Width, height), d.Rect, gfx.PathStyle{Stroke: color, StrokeWidth: 1.5, Cap: gfx.CapRound})
	}
	for _, edge := range buffer.Edges {
		color := apptheme.GraphColor(edge.ColorId, dark)
		x := 8 + float32(edge.PositionInCurrentRow)*16
		other := 8 + float32(edge.PositionInOtherRow)*16
		endY := float32(0)
		if edge.EdgePrintType == printer.EdgeDown {
			endY = height
		}
		endX := (x + other) / 2
		if edge.LineStyle == printer.LineSolid || edge.HasArrow {
			line(x, middle, endX, endY, color)
		} else {
			line(x, middle, (x+endX)/2, (middle+endY)/2, color)
		}
		if edge.HasArrow {
			sign := float32(1)
			if endY < middle {
				sign = -1
			}
			line(endX, endY, endX-3, endY-sign*4, color)
			line(endX, endY, endX+3, endY-sign*4, color)
		}
	}
	for _, node := range buffer.Nodes {
		color := apptheme.GraphColor(node.ColorId, dark)
		x := d.Rect.X + 8 + float32(node.PositionInCurrentRow)*16
		d.Canvas.Rounded(gfx.Rect{X: x - 4, Y: d.Rect.Y + middle - 4, Width: 8, Height: 8}, 4, color)
	}
}

func shortDate(when time.Time) string { return when.Format("2/1/06, 3:04 pm") }
func (v *View) selectedCommit() (model.Commit, bool) {
	for _, commit := range v.Commits {
		if commit.Hash.Equal(v.selected) {
			return commit, true
		}
	}
	return model.Commit{}, false
}

// SetContaining installs branch containment from the repository, so branch
// filters and metadata pagination do not change the branch membership count.
func (v *View) SetContaining(names []string) {
	v.branchLine = fmt.Sprintf("In %d branches: %s", len(names), strings.Join(names, ", "))
}
