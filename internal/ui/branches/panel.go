// Package branches is the branches side panel and the dialogs that act on refs.
package branches

import (
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/vivaui/viva/ui"
)

// NodeKind classifies one visible row of the branches tree.
type NodeKind uint8

const (
	KindGroup NodeKind = iota
	KindBranch
	KindTag
)

// Node is one flattened tree row. Groups are branches; leaves are refs.
type Node struct {
	Kind     NodeKind
	Label    string
	Depth    int
	Expanded bool
	Current  bool
	Favorite bool
	Ahead    int
	Behind   int
	// Ref is the short name for a branch or tag. Group rows leave it empty.
	Ref string
	// Remote is the remote name for a remote-tracking branch.
	Remote string
	// Group is the group key a leaf belongs to: "local", "tags", or "remote:<name>".
	Group string
}

// Action is a context-menu or toolbar command the panel reports to the shell.
type Action uint8

const (
	ActionNone Action = iota
	ActionCheckout
	ActionNewBranch
	ActionRename
	ActionDelete
	ActionMerge
	ActionRebase
	ActionCompare
	ActionCheckoutRebase
	ActionFavorite
	ActionPush
	ActionPull
	ActionFetch
	ActionStash
	ActionRemotes
)

// Request is one user action, delivered on the paint that clicked it.
type Request struct {
	Action Action
	Ref    string
	Remote string
	Kind   NodeKind
}

// Panel is the branches, remotes, and tags tree.
//
// The shell fills Nodes from a repository snapshot. The panel paints them,
// expands groups, and reports actions. Git runs in the shell, which owns
// cancellation.
type Panel struct {
	Nodes    []Node
	Filter   string
	Selected int
	Error    string

	tree      ui.TreeState
	visible   []int
	expanded  map[string]bool
	page      ui.C
	actions   bool
	OnRequest func(Request)
}

// Load replaces the tree from a repository snapshot.
// Current-branch ahead/behind comes from the snapshot. Other local branches
// use aheadBehind, keyed by short branch name. favorites lists short names
// the user marked.
func (p *Panel) Load(snap repo.Snapshot, aheadBehind map[string][2]int, favorites []string) {
	fav := map[string]bool{}
	for _, name := range favorites {
		fav[name] = true
	}
	if p.expanded == nil {
		p.expanded = map[string]bool{"local": true}
	}
	var nodes []Node
	nodes = append(nodes, Node{Kind: KindGroup, Label: "Local", Group: "local", Depth: 0, Expanded: p.expanded["local"]})
	current := snap.CurrentBranch.Name()
	for _, ref := range snap.Branches {
		if ref.Ref.Kind() != model.KindLocalBranch {
			continue
		}
		name := ref.Ref.Name()
		counts := aheadBehind[name]
		if name == current {
			counts = [2]int{snap.Ahead, snap.Behind}
		}
		nodes = append(nodes, Node{
			Kind: KindBranch, Label: name, Ref: name, Group: "local", Depth: 1,
			Current: name == current, Favorite: fav[name], Ahead: counts[0], Behind: counts[1],
		})
	}
	byRemote := map[string][]repo.Reference{}
	var remoteOrder []string
	for _, ref := range snap.Branches {
		if ref.Ref.Kind() != model.KindRemoteBranch {
			continue
		}
		parsed := model.ParseRemoteBranch(ref.Ref.Name(), snap.Remotes)
		remote := parsed.Remote.Name
		if _, ok := byRemote[remote]; !ok {
			remoteOrder = append(remoteOrder, remote)
		}
		byRemote[remote] = append(byRemote[remote], ref)
	}
	for _, remote := range remoteOrder {
		group := "remote:" + remote
		nodes = append(nodes, Node{Kind: KindGroup, Label: remote, Group: group, Remote: remote, Depth: 0, Expanded: p.expanded[group]})
		for _, ref := range byRemote[remote] {
			parsed := model.ParseRemoteBranch(ref.Ref.Name(), snap.Remotes)
			nodes = append(nodes, Node{
				Kind: KindBranch, Label: parsed.NameForRemoteOperations, Ref: ref.Ref.Name(),
				Remote: remote, Group: group, Depth: 1,
			})
		}
	}
	nodes = append(nodes, Node{Kind: KindGroup, Label: "Tags", Group: "tags", Depth: 0, Expanded: p.expanded["tags"]})
	for _, tag := range snap.Tags {
		nodes = append(nodes, Node{Kind: KindTag, Label: tag.Ref.Name(), Ref: tag.Ref.Name(), Group: "tags", Depth: 1})
	}
	p.Nodes = nodes
	if p.Selected >= len(nodes) {
		p.Selected = 0
	}
}

// Page paints the tree and returns the action clicked this paint.
func (p *Panel) Page(c ui.C) Request {
	p.page = c
	p.rebuildVisible()
	p.tree.Node = p.node
	p.tree.Key = p.key
	var req Request
	ui.Column(c).Fill().Gap(8).Pad(8).Children(func() {
		ui.Text(c, "Branches").Bold()
		ui.TextField(c, &p.Filter, "Find branch", "Type to filter")
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			if ui.Button(c, "Fetch remote").Clicked() {
				req.Action = ActionFetch
			}
			if ui.Button(c, "Pull changes").Clicked() {
				req.Action = ActionPull
			}
			if ui.Button(c, "Push branches").Clicked() {
				req.Action = ActionPush
			}
		})
		ui.Row(c).FillWidth().Gap(4).Children(func() {
			if ui.Button(c, "Stash").Clicked() {
				req.Action = ActionStash
			}
			if ui.Button(c, "Remotes").Clicked() {
				req.Action = ActionRemotes
			}
		})
		if p.Error != "" {
			ui.Text(c, p.Error).Color(c.Theme().Danger).FontSize(12)
		}
		tree := ui.Column(c).Grow(1).Hit(ui.ID(c), "Branches").Children(func() {
			ui.Tree(c, &p.tree, len(p.visible), p.label, p.row).Grow(1)
		})
		ui.ContextMenu(c, tree, func(menu *ui.MenuItems) {
			req = p.menu(menu, req)
		})
		anchor := ui.Button(c, "Branch actions")
		if anchor.Clicked() {
			p.actions = !p.actions
		}
		ui.Menu(c, anchor, &p.actions, func(menu *ui.MenuItems) {
			req = p.menu(menu, req)
		})
	})
	if i, ok := p.tree.Toggled(); ok && i < len(p.visible) {
		p.toggle(i)
	}
	if i, ok := p.tree.Chosen(); ok && i < len(p.visible) {
		p.Selected = p.visible[i]
	}
	if req.Action != ActionNone && p.OnRequest != nil {
		captured := req
		c.After(func() { p.OnRequest(captured) })
	}
	return req
}

func (p *Panel) menu(menu *ui.MenuItems, req Request) Request {
	node, ok := p.selectedNode()
	if !ok || node.Kind == KindGroup {
		return req
	}
	if node.Kind == KindBranch && !node.Current && menu.Item("Checkout").Clicked() {
		return Request{Action: ActionCheckout, Ref: node.Ref, Remote: node.Remote, Kind: node.Kind}
	}
	if node.Kind == KindBranch && menu.Item("New branch from").Clicked() {
		return Request{Action: ActionNewBranch, Ref: node.Ref, Remote: node.Remote, Kind: node.Kind}
	}
	if node.Kind == KindBranch && node.Remote == "" && menu.Item("Rename").Clicked() {
		return Request{Action: ActionRename, Ref: node.Ref, Kind: node.Kind}
	}
	if !node.Current && menu.Item("Delete").Clicked() {
		return Request{Action: ActionDelete, Ref: node.Ref, Remote: node.Remote, Kind: node.Kind}
	}
	if node.Kind == KindBranch && !node.Current && menu.Item("Merge into current").Clicked() {
		return Request{Action: ActionMerge, Ref: node.Ref, Kind: node.Kind}
	}
	if node.Kind == KindBranch && !node.Current && menu.Item("Rebase current onto").Clicked() {
		return Request{Action: ActionRebase, Ref: node.Ref, Kind: node.Kind}
	}
	if menu.Item("Compare").Clicked() {
		return Request{Action: ActionCompare, Ref: node.Ref, Kind: node.Kind}
	}
	if node.Kind == KindBranch && node.Remote != "" && menu.Item("Checkout and rebase").Clicked() {
		return Request{Action: ActionCheckoutRebase, Ref: node.Ref, Remote: node.Remote, Kind: node.Kind}
	}
	if node.Kind == KindBranch && node.Remote == "" {
		label := "Favorite"
		if node.Favorite {
			label = "Unfavorite"
		}
		if menu.Item(label).Clicked() {
			return Request{Action: ActionFavorite, Ref: node.Ref, Kind: node.Kind}
		}
	}
	return req
}

func (p *Panel) rebuildVisible() {
	p.visible = p.visible[:0]
	hide := map[string]bool{}
	needle := strings.ToLower(strings.TrimSpace(p.Filter))
	for i, node := range p.Nodes {
		if node.Kind == KindGroup {
			if !node.Expanded {
				hide[node.Group] = true
			}
			if needle == "" {
				p.visible = append(p.visible, i)
			}
			continue
		}
		if hide[node.Group] && needle == "" {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(node.Label), needle) && !strings.Contains(strings.ToLower(node.Ref), needle) {
			continue
		}
		p.visible = append(p.visible, i)
	}
	if needle == "" {
		return
	}
	seen := map[string]bool{}
	withGroups := make([]int, 0, len(p.visible)*2)
	for _, i := range p.visible {
		group := p.Nodes[i].Group
		if !seen[group] {
			for j, node := range p.Nodes {
				if node.Kind == KindGroup && node.Group == group {
					withGroups = append(withGroups, j)
					break
				}
			}
			seen[group] = true
		}
		withGroups = append(withGroups, i)
	}
	p.visible = withGroups
}

func (p *Panel) node(i int) ui.TreeNode {
	node := p.Nodes[p.visible[i]]
	return ui.TreeNode{
		Depth:    node.Depth,
		Branch:   node.Kind == KindGroup,
		Expanded: node.Expanded,
		Selected: p.visible[i] == p.Selected,
	}
}

func (p *Panel) key(i int) int { return p.visible[i] }

func (p *Panel) label(i int) string {
	node := p.Nodes[p.visible[i]]
	if node.Current {
		return node.Label + " (current)"
	}
	return node.Label
}

func (p *Panel) row(i int) {
	c := p.page
	node := p.Nodes[p.visible[i]]
	text := p.label(i)
	if node.Favorite {
		text += " ★"
	}
	ui.Text(c, text)
	if badge := BadgeText(node.Ahead, node.Behind); badge != "" {
		ui.Text(c, badge).FontSize(12).Muted()
	}
}

func (p *Panel) toggle(visible int) {
	node := &p.Nodes[p.visible[visible]]
	node.Expanded = !node.Expanded
	if p.expanded == nil {
		p.expanded = map[string]bool{}
	}
	p.expanded[node.Group] = node.Expanded
}

func (p *Panel) selectedNode() (Node, bool) {
	if p.Selected < 0 || p.Selected >= len(p.Nodes) {
		return Node{}, false
	}
	return p.Nodes[p.Selected], true
}

// SelectedNode returns the row the user last chose.
func (p *Panel) SelectedNode() (Node, bool) { return p.selectedNode() }

// VisibleCount is the number of rows the tree currently shows.
func (p *Panel) VisibleCount() int {
	p.rebuildVisible()
	return len(p.visible)
}

// BadgeText formats the ahead/behind marker the row paints.
func BadgeText(ahead, behind int) string {
	if ahead == 0 && behind == 0 {
		return ""
	}
	return fmt.Sprintf("↑%d ↓%d", ahead, behind)
}

// ForcePushAllowed reports whether force-with-lease may be offered for branch.
// Each pattern matches the whole branch name, as GitSharedSettings.isMatchSafe does.
func ForcePushAllowed(branch string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == branch {
			return false
		}
	}
	return true
}

// RejectedNonFF reports whether any push result is a non-fast-forward rejection
// that "Pull then push" can recover.
func RejectedNonFF(results []ops.PushNativeResult) bool {
	for _, result := range results {
		if result.IsNonFFUpdate() {
			return true
		}
	}
	return false
}
