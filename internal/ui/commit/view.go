// Package commit is the local-changes tree and commit panel, the Commit
// tool window. Diff display is a callback; the diff UI is a later change.
package commit

import (
	"github.com/runbrick/r-git/internal/i18n"
	"path"
	"sort"
	"strings"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/runbrick/r-git/internal/ui/controls"
	"github.com/runbrick/r-git/internal/ui/icons"
	"github.com/vivaui/viva/ui"
)

// NodeKind is one row of the flattened changes tree.
type NodeKind int

const (
	NodeGroup NodeKind = iota
	NodeDir
	NodeFile
)

// Node is one visible row. Children of a collapsed branch are omitted.
type Node struct {
	countText   i18n.NumberText
	includeText i18n.NameText
	Count       int
	Kind        NodeKind
	Group       repo.ChangeGroup
	Depth       int
	Label       string
	Path        string
	Status      model.FileStatus
	Included    bool
	Branch      bool
	Expanded    bool
}

// Request is one action the panel asks the shell to run. The view does
// not call git itself.
type Request struct {
	Kind     RequestKind
	Paths    []string
	Message  string
	Amend    bool
	SignOff  bool
	Author   string
	Push     bool
	DiffPath string
}

// RequestKind names the operation behind a Request.
type RequestKind int

const (
	RequestNone RequestKind = iota
	RequestStage
	RequestUnstage
	RequestDiscard
	RequestCommit
	RequestShowDiff
)

// View is the changes tree and the commit form.
type View struct {
	discardText i18n.NameText
	i18n.Locale
	Entries     []repo.ChangeEntry
	Busy        bool
	Error       string
	OnRefresh   func()
	optionsOpen bool
	amendMenu   bool

	// OnRequest runs after the paint, on the window thread. The shell
	// starts the git work and publishes the result.
	OnRequest func(Request)
	// OnShowDiff is the shell's diff callback (hash, path). The hash is
	// zero: the diff is of the working tree, and the diff UI is later.
	OnShowDiff func(model.Hash, string)

	Message string
	Amend   bool
	SignOff bool
	Author  string

	tree     ui.TreeState
	nodes    []Node
	expanded map[string]bool
	included map[string]bool
	rowC     ui.C
	menuPath string
	menuNode int

	discardOpen bool
	discardPath string

	lastMessage string
	amendWas    bool
	savedDraft  string
	toaster     ui.Toaster
	built       bool
}

// SetChanges replaces the file list. Inclusion of a path that is still
// present is kept; a newly seen path is included.
func (v *View) SetChanges(entries []repo.ChangeEntry) {
	if v.included == nil {
		v.included = map[string]bool{}
	}
	if v.expanded == nil {
		v.expanded = map[string]bool{}
	}
	next := make(map[string]bool, len(entries))
	for _, entry := range entries {
		key := includeKey(entry.Group, entry.Path)
		if on, ok := v.included[key]; ok {
			next[key] = on
		} else {
			next[key] = true
		}
	}
	v.included = next
	v.Entries = entries
	v.rebuild()
}

// SetIncluded marks whether a path in a group is part of the next commit.
func (v *View) SetIncluded(group repo.ChangeGroup, path string, on bool) {
	if v.included == nil {
		v.included = map[string]bool{}
	}
	v.included[includeKey(group, path)] = on
	v.rebuild()
}

// SetLastMessage records HEAD's message so Amend can prefill it.
func (v *View) SetLastMessage(message string) {
	v.lastMessage = strings.TrimRight(message, "\n")
	if i := strings.IndexByte(v.lastMessage, '\n'); i >= 0 {
		v.lastMessage = strings.TrimRight(v.lastMessage[:i], "\r")
	}
}

// ShowToast queues a toast painted on the next Page call.
func (v *View) ShowToast(title, description string, kind ui.ToastKind) {
	v.toaster.Show(title, description, kind)
}

// Page declares the changes tree and the commit form side by side.
func (v *View) Page(c ui.C) {
	if !v.built {
		v.built = true
		v.tree.Node = v.nodeAt
		v.tree.Key = v.nodeKey
		v.tree.Label = v.rowLabel
		v.tree.RowHeight = 22
	}
	v.rowC = c
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(32).FillWidth().PadX(12).Items(ui.Center).Border(1, c.Theme().Border).Children(func() { ui.Text(c, v.Text().Commit).FontSize(13) })
		v.treePane(c)
		ui.Divider(c)
		v.formPane(c)
	})
	ui.Toasts(c, &v.toaster)
	v.discardModal(c)
}

func (v *View) treePane(c ui.C) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Grow(1).Children(func() {
		ui.Row(c).FillWidth().Height(36).PadX(8).Gap(4).Items(ui.Center).Background(theme.Background).Children(func() {
			if controls.IconButton(c, icons.Pair(c, icons.Refresh, icons.Refresh_dark), v.Text().RefreshChanges).Disabled(v.OnRefresh == nil || v.Busy).Clicked() {
				c.After(v.OnRefresh)
			}
			if node, ok := v.actionNode(); ok {
				v.rowActions(c, node)
			}
			ui.Spacer(c)
			options := controls.IconButton(c, icons.Pair(c, icons.More, icons.More_dark), v.Text().CommitOptions)
			if options.Clicked() {
				v.optionsOpen = !v.optionsOpen
			}
			ui.Popover(c, options, &v.optionsOpen, func() {
				ui.Column(c).Width(250).Pad(8).Gap(8).Children(func() {
					ui.Checkbox(c, &v.SignOff, v.Text().SignOff)
					ui.TextField(c, &v.Author, v.Text().Author, v.Text().NameEmail)
				})
			})
		})
		if len(v.nodes) == 0 {
			ui.Column(c).Fill().Pad(16).Children(func() {
				ui.Text(c, v.Text().NoChanges).Muted()
			})
			return
		}
		ui.Tree(c, &v.tree, len(v.nodes), v.rowLabel, v.rowContent).Grow(1)
		if i, ok := v.tree.Chosen(); ok {
			c.After(func() { v.tree.Select(i) })
		}
		if i, ok := v.tree.Toggled(); ok {
			v.toggle(i)
		}
	})
}

func (v *View) rowLabel(i int) string {
	if i < 0 || i >= len(v.nodes) {
		return ""
	}
	return v.nodes[i].Label
}

func (v *View) rowContent(i int) {
	node := v.nodes[i]
	c := v.rowC
	theme := c.Theme()
	switch node.Kind {
	case NodeGroup, NodeDir:
		on := node.Included
		if ui.Checkbox(c.Key(branchKey(node)), &on, "").Label(v.nodes[i].includeText.Get(v.Text(), node.Label, v.Text().IncludePath)).Changed() {
			for _, entry := range v.Entries {
				if entry.Group == node.Group && (node.Kind == NodeGroup || strings.HasPrefix(entry.Path, node.Path+"/")) {
					v.included[includeKey(entry.Group, entry.Path)] = on
				}
			}
			c.After(v.rebuild)
		}
		if node.Kind == NodeDir {
			ui.Icon(c, icons.Pair(c, icons.Folder, icons.Folder_dark)).Size(16, 16)
		}
		text := ui.Text(c, node.Label).FontSize(13)
		if node.Kind == NodeGroup {
			text.Bold()
		}
		ui.Text(c, v.nodes[i].countText.Get(v.Text(), node.Count, v.Text().FilesCount)).FontSize(12).Muted()
	default:
		on := node.Included
		if ui.Checkbox(c.Key(includeKey(node.Group, node.Path)), &on, "").Label(v.nodes[i].includeText.Get(v.Text(), node.Path, v.Text().IncludePath)).Changed() {
			v.included[includeKey(node.Group, node.Path)] = on
			c.After(v.rebuild)
		}
		ui.Icon(c, icons.File(c, node.Path)).Size(16, 16)
		row := ui.Row(c).Hit(ui.ID(c), node.Path).Gap(4).Items(ui.Center).Grow(1).Children(func() {
			ui.Text(c, node.Label).FontSize(13).Color(*statusColor(theme, node.Status)).Ellipsis().Grow(1)
		})
		if row.Clicked() {
			c.After(func() { v.tree.Select(i) })
		}
		ui.ContextMenu(c, row, func(menu *ui.MenuItems) { v.menuFor(menu, node) })
	}
}

func (v *View) actionNode() (Node, bool) {
	if i := v.tree.Selected(); i >= 0 && i < len(v.nodes) && v.nodes[i].Kind == NodeFile {
		return v.nodes[i], true
	}
	for _, node := range v.nodes {
		if node.Kind == NodeFile {
			return node, true
		}
	}
	return Node{}, false
}

func (v *View) rowActions(c ui.C, node Node) {
	if node.Group != repo.GroupStaged && node.Group != repo.GroupConflicts {
		if controls.IconButton(c.Key(node.Path+"-stage"), icons.Pair(c, icons.Add, icons.Add_dark), v.Text().Stage).Clicked() {
			v.emit(Request{Kind: RequestStage, Paths: []string{node.Path}})
		}
	}
	if node.Group == repo.GroupStaged {
		if controls.IconButton(c.Key(node.Path+"-unstage"), icons.Pair(c, icons.Remove, icons.Remove_dark), v.Text().Unstage).Clicked() {
			v.emit(Request{Kind: RequestUnstage, Paths: []string{node.Path}})
		}
	}
	if controls.IconButton(c.Key(node.Path+"-rollback"), icons.Pair(c, icons.Rollback, icons.Rollback_dark), v.Text().Rollback).Clicked() {
		v.discardPath = node.Path
		v.discardOpen = true
	}
	if controls.IconButton(c.Key(node.Path+"-diff"), icons.Pair(c, icons.Diff, icons.Diff_dark), v.Text().Diff).Clicked() {
		if v.OnShowDiff != nil {
			v.OnShowDiff(model.Hash{}, node.Path)
		}
		v.emit(Request{Kind: RequestShowDiff, DiffPath: node.Path})
	}
}

func (v *View) menuFor(menu *ui.MenuItems, node Node) {
	v.menuPath = node.Path
	if node.Group != repo.GroupStaged && node.Group != repo.GroupConflicts {
		if menu.Item(v.Text().Stage).Clicked() {
			v.emit(Request{Kind: RequestStage, Paths: []string{node.Path}})
		}
	}
	if node.Group == repo.GroupStaged {
		if menu.Item(v.Text().Unstage).Clicked() {
			v.emit(Request{Kind: RequestUnstage, Paths: []string{node.Path}})
		}
	}
	if menu.Item(v.Text().Rollback).Clicked() {
		v.discardPath = node.Path
		v.discardOpen = true
	}
	if menu.Item(v.Text().ShowDiff).Clicked() {
		if v.OnShowDiff != nil {
			v.OnShowDiff(model.Hash{}, node.Path)
		}
		v.emit(Request{Kind: RequestShowDiff, DiffPath: node.Path})
	}
}

func (v *View) discardModal(c ui.C) {
	ui.ModalWith(c, &v.discardOpen, v.Text().RollbackChanges, ui.ModalSpec{NoClose: v.Text() != &i18n.En}, func() {
		ui.Text(c, v.discardText.Get(v.Text(), v.discardPath, v.Text().DiscardPrompt))
	}, func() {
		if ui.Button(c, v.Text().Cancel).Outline().Clicked() {
			v.discardOpen = false
		}
		if ui.Button(c, v.Text().Discard).Clicked() {
			file := v.discardPath
			v.discardOpen = false
			v.emit(Request{Kind: RequestDiscard, Paths: []string{file}})
		}
	})
}

func (v *View) formPane(c ui.C) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Height(320).Pad(12).Gap(8).Background(theme.Background).Children(func() {
		ui.Row(c).FillWidth().Gap(4).Items(ui.Center).Children(func() {
			ui.Checkbox(c, &v.Amend, v.Text().Amend)
			last := ui.ButtonBase(c, v.Text().LastCommit).Height(28).PadX(4).Gap(4).Tooltip(v.Text().AmendLastCommit).Children(func() {
				ui.Text(c, v.Text().LastCommit).FontSize(13).Color(theme.Primary)
				ui.Icon(c, icons.Pair(c, icons.Down, icons.Down_dark)).Size(16, 16)
			})
			if last.Clicked() {
				v.amendMenu = !v.amendMenu
			}
			ui.Menu(c, last, &v.amendMenu, func(menu *ui.MenuItems) {
				if menu.Item(v.Text().AmendLastCommit).Clicked() {
					v.Amend = true
				}
			})
		})
		v.syncAmend()
		ui.TextArea(c, &v.Message, v.Text().CommitMessage, v.Text().DescribeTheChange).Mono().FontSize(13).Grow(1).FillWidth()
		if v.Error != "" {
			ui.Text(c, v.Error).FontSize(12).Color(theme.Danger)
		}
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			commit := controls.TextButton(c, v.Text().CommitChanges, v.Text().Commit).Disabled(v.Busy)
			push := controls.TextButton(c, v.Text().CommitAndPush, v.Text().CommitAndPushEllipsis).Disabled(v.Busy)
			if commit.Clicked() {
				v.submit(false)
			}
			if push.Clicked() {
				v.submit(true)
			}
		})
		if ui.Shortcut(c, ui.Primary, "enter") {
			v.submit(false)
		}
	})
}

func (v *View) submit(push bool) {
	if v.Busy {
		return
	}
	if strings.TrimSpace(v.Message) == "" {
		v.Error = v.Text().EnterACommitMessage
		return
	}
	v.Error = ""
	v.emit(Request{
		Kind:    RequestCommit,
		Paths:   v.includedPaths(),
		Message: v.Message,
		Amend:   v.Amend,
		SignOff: v.SignOff,
		Author:  strings.TrimSpace(v.Author),
		Push:    push,
	})
}

// ResetAfterCommit leaves the tool window open with an empty draft and Amend
// unchecked. Author and Sign-off remain selected, as in IntelliJ's commit panel.
func (v *View) ResetAfterCommit() {
	v.Message = ""
	v.Amend = false
	v.amendWas = false
	v.savedDraft = ""
	v.Error = ""
}

func (v *View) syncAmend() {
	if v.Amend == v.amendWas {
		return
	}
	if v.Amend {
		v.savedDraft = v.Message
		if strings.TrimSpace(v.Message) == "" {
			v.Message = v.lastMessage
		}
	} else {
		v.Message = v.savedDraft
	}
	v.amendWas = v.Amend
}

func (v *View) emit(req Request) {
	if v.OnRequest == nil {
		return
	}
	v.OnRequest(req)
}

func (v *View) includedPaths() []string {
	seen := map[string]bool{}
	var paths []string
	for _, entry := range v.Entries {
		if entry.Group == repo.GroupConflicts {
			continue
		}
		if !v.included[includeKey(entry.Group, entry.Path)] {
			continue
		}
		if seen[entry.Path] {
			continue
		}
		seen[entry.Path] = true
		paths = append(paths, entry.Path)
	}
	sort.Strings(paths)
	return paths
}

func (v *View) toggle(i int) {
	if i < 0 || i >= len(v.nodes) || !v.nodes[i].Branch {
		return
	}
	key := branchKey(v.nodes[i])
	v.expanded[key] = !v.expanded[key]
	v.rebuild()
}

func (v *View) nodeAt(i int) ui.TreeNode {
	if i < 0 || i >= len(v.nodes) {
		return ui.TreeNode{}
	}
	n := v.nodes[i]
	return ui.TreeNode{Depth: n.Depth, Branch: n.Branch, Expanded: n.Expanded, Selected: v.tree.Selected() == i}
}

func (v *View) nodeKey(i int) int {
	if i < 0 || i >= len(v.nodes) {
		return i
	}
	return keyOf(branchKey(v.nodes[i]))
}

func (v *View) rebuild() {
	if v.expanded == nil {
		v.expanded = map[string]bool{}
	}
	v.nodes = v.nodes[:0]
	groups := []repo.ChangeGroup{repo.GroupStaged, repo.GroupUnstaged, repo.GroupUnversioned, repo.GroupConflicts}
	for _, group := range groups {
		var files []repo.ChangeEntry
		for _, entry := range v.Entries {
			if entry.Group == group {
				files = append(files, entry)
			}
		}
		if len(files) == 0 {
			continue
		}
		sort.Slice(files, func(a, b int) bool { return files[a].Path < files[b].Path })
		groupNode := Node{Kind: NodeGroup, Group: group, Label: v.groupLabel(group), Count: len(files), Included: v.allIncluded(group, files), Branch: true, Path: group.String()}
		groupNode.Expanded = v.isOpen(groupNode)
		v.nodes = append(v.nodes, groupNode)
		if !groupNode.Expanded {
			continue
		}
		v.appendDirs(group, files, "", 1)
	}
}

func (v *View) appendDirs(group repo.ChangeGroup, files []repo.ChangeEntry, prefix string, depth int) {
	type bucket struct {
		name  string
		files []repo.ChangeEntry
	}
	var here []repo.ChangeEntry
	dirs := map[string]*bucket{}
	var order []string
	for _, file := range files {
		rel := file.Path
		if prefix != "" {
			rel = strings.TrimPrefix(file.Path, prefix+"/")
		}
		dir, _, ok := strings.Cut(rel, "/")
		if !ok {
			here = append(here, file)
			continue
		}
		b := dirs[dir]
		if b == nil {
			b = &bucket{name: dir}
			dirs[dir] = b
			order = append(order, dir)
		}
		b.files = append(b.files, file)
	}
	sort.Strings(order)
	for _, name := range order {
		b := dirs[name]
		full := name
		if prefix != "" {
			full = prefix + "/" + name
		}
		dirNode := Node{Kind: NodeDir, Group: group, Depth: depth, Label: name, Count: len(b.files), Included: v.allIncluded(group, b.files), Path: full, Branch: true}
		dirNode.Expanded = v.isOpen(dirNode)
		v.nodes = append(v.nodes, dirNode)
		if dirNode.Expanded {
			v.appendDirs(group, b.files, full, depth+1)
		}
	}
	for _, file := range here {
		v.nodes = append(v.nodes, Node{
			Kind:     NodeFile,
			Group:    group,
			Depth:    depth,
			Label:    path.Base(file.Path),
			Path:     file.Path,
			Status:   file.Status,
			Included: v.included[includeKey(group, file.Path)],
		})
	}
}

func (v *View) isOpen(n Node) bool {
	key := branchKey(n)
	open, ok := v.expanded[key]
	if !ok {
		v.expanded[key] = true
		return true
	}
	return open
}

func branchKey(n Node) string {
	return n.Group.String() + "\x00" + n.Path
}

func includeKey(group repo.ChangeGroup, filePath string) string {
	return group.String() + "\x00" + filePath
}

func keyOf(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = h*31 + int(s[i])
	}
	if h < 0 {
		h = -h
	}
	return h
}

func countLabel(n int) string {
	if n == 1 {
		return "1 file"
	}
	return itoa(n) + " files"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func statusColor(theme *ui.Theme, status model.FileStatus) *ui.Color {
	switch status {
	case model.StatusAdded, model.StatusCopied, model.StatusUntracked:
		return &theme.Success
	case model.StatusDeleted:
		return &theme.Danger
	case model.StatusModified, model.StatusTypeChanged, model.StatusRenamed:
		return &theme.Primary
	case model.StatusUnmerged:
		return &theme.Danger
	default:
		return &theme.Foreground
	}
}

func (v *View) groupLabel(group repo.ChangeGroup) string {
	switch group {
	case repo.GroupUnstaged:
		return v.Text().Changes
	case repo.GroupUnversioned:
		return v.Text().UnversionedFiles
	case repo.GroupStaged:
		return v.Text().Staged
	case repo.GroupConflicts:
		return v.Text().Conflicts
	default:
		return group.String()
	}
}
func (v *View) allIncluded(group repo.ChangeGroup, files []repo.ChangeEntry) bool {
	for _, entry := range files {
		if !v.included[includeKey(group, entry.Path)] {
			return false
		}
	}
	return len(files) > 0
}

// SetMessages refreshes translated group rows while retaining inclusion state.
func (v *View) SetMessages(messages *i18n.Messages) { v.UseMessages(messages); v.rebuild() }
