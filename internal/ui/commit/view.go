// Package commit is the local-changes tree and commit panel, the Commit
// tool window. Diff display is a callback; the diff UI is a later change.
package commit

import (
	"path"
	"sort"
	"strings"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
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
	Kind     NodeKind
	Group    repo.ChangeGroup
	Depth    int
	Label    string
	Path     string
	Status   model.FileStatus
	Included bool
	Branch   bool
	Expanded bool
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
	Entries []repo.ChangeEntry
	Busy    bool
	Error   string

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
		v.tree.RowHeight = 28
	}
	v.rowC = c
	ui.Row(c).Fill().Children(func() {
		v.treePane(c)
		ui.Divider(c)
		v.formPane(c)
	})
	ui.Toasts(c, &v.toaster)
	v.discardModal(c)
}

func (v *View) treePane(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(460).FillHeight().Children(func() {
		ui.Row(c).FillWidth().PadX(12).PadY(8).Background(theme.Card).Children(func() {
			ui.Text(c, "Changes").Bold()
			ui.Spacer(c)
			ui.Text(c, countLabel(len(v.Entries))).FontSize(12).Muted()
		})
		if len(v.nodes) == 0 {
			ui.Column(c).Fill().Pad(16).Children(func() {
				ui.Text(c, "No changes").Muted()
			})
			return
		}
		ui.Tree(c, &v.tree, len(v.nodes), v.rowLabel, v.rowContent).Grow(1)
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
	case NodeGroup:
		ui.Text(c, node.Label).Bold().FontSize(12)
	case NodeDir:
		ui.Text(c, node.Label).FontSize(13)
	default:
		on := node.Included
		box := ui.Checkbox(c.Key(includeKey(node.Group, node.Path)), &on, "")
		if box.Changed() {
			v.included[includeKey(node.Group, node.Path)] = on
			v.nodes[i].Included = on
		}
		ui.Text(c, node.Status.Letter()).Width(16).FontSize(12).Bold().Color(*statusColor(theme, node.Status))
		row := ui.Row(c).Hit(ui.ID(c), node.Path).Gap(8).Items(ui.Center).Grow(1).Children(func() {
			ui.Text(c, node.Label).FontSize(13).Color(*statusColor(theme, node.Status))
			ui.Spacer(c)
			v.rowActions(c, node)
		})
		ui.ContextMenu(c, row, func(menu *ui.MenuItems) {
			v.menuFor(menu, node)
		})
	}
}

func (v *View) rowActions(c ui.C, node Node) {
	if node.Group != repo.GroupStaged && node.Group != repo.GroupConflicts {
		if ui.Button(c.Key(node.Path+"-stage"), "Stage").Clicked() {
			v.emit(Request{Kind: RequestStage, Paths: []string{node.Path}})
		}
	}
	if node.Group == repo.GroupStaged {
		if ui.Button(c.Key(node.Path+"-unstage"), "Unstage").Clicked() {
			v.emit(Request{Kind: RequestUnstage, Paths: []string{node.Path}})
		}
	}
	if ui.Button(c.Key(node.Path+"-rollback"), "Rollback").Clicked() {
		v.discardPath = node.Path
		v.discardOpen = true
	}
	if ui.Button(c.Key(node.Path+"-diff"), "Diff").Clicked() {
		if v.OnShowDiff != nil {
			v.OnShowDiff(model.Hash{}, node.Path)
		}
		v.emit(Request{Kind: RequestShowDiff, DiffPath: node.Path})
	}
}

func (v *View) menuFor(menu *ui.MenuItems, node Node) {
	v.menuPath = node.Path
	if node.Group != repo.GroupStaged && node.Group != repo.GroupConflicts {
		if menu.Item("Stage").Clicked() {
			v.emit(Request{Kind: RequestStage, Paths: []string{node.Path}})
		}
	}
	if node.Group == repo.GroupStaged {
		if menu.Item("Unstage").Clicked() {
			v.emit(Request{Kind: RequestUnstage, Paths: []string{node.Path}})
		}
	}
	if menu.Item("Rollback").Clicked() {
		v.discardPath = node.Path
		v.discardOpen = true
	}
	if menu.Item("Show diff").Clicked() {
		if v.OnShowDiff != nil {
			v.OnShowDiff(model.Hash{}, node.Path)
		}
		v.emit(Request{Kind: RequestShowDiff, DiffPath: node.Path})
	}
}

func (v *View) discardModal(c ui.C) {
	ui.Modal(c, &v.discardOpen, "Rollback changes", func() {
		ui.Text(c, "Discard uncommitted changes to "+v.discardPath+"? This cannot be undone.")
	}, func() {
		if ui.Button(c, "Cancel").Clicked() {
			v.discardOpen = false
		}
		if ui.Button(c, "Discard").Clicked() {
			file := v.discardPath
			v.discardOpen = false
			v.emit(Request{Kind: RequestDiscard, Paths: []string{file}})
		}
	})
}

func (v *View) formPane(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Pad(16).Gap(12).Background(theme.Card).Children(func() {
		ui.Text(c, "Commit").FontSize(16).Bold()
		v.syncAmend()
		ui.TextArea(c, &v.Message, "Commit message", "Describe the change").Height(140)
		if v.Error != "" {
			ui.Text(c, v.Error).FontSize(12).Color(theme.Danger)
		}
		ui.Checkbox(c, &v.Amend, "Amend")
		ui.Checkbox(c, &v.SignOff, "Sign-off")
		ui.TextField(c, &v.Author, "Author", "Name <email>")
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			commit := ui.Button(c, "Commit changes").Disabled(v.Busy)
			push := ui.Button(c, "Commit and Push").Disabled(v.Busy)
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
		v.Error = "Enter a commit message."
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
	return ui.TreeNode{Depth: n.Depth, Branch: n.Branch, Expanded: n.Expanded}
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
		groupNode := Node{Kind: NodeGroup, Group: group, Label: group.String(), Branch: true, Path: group.String()}
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
		dirNode := Node{Kind: NodeDir, Group: group, Depth: depth, Label: name, Path: full, Branch: true}
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
