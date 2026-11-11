package branches

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/git/repo"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// Session owns branch, push, pull, stash, and remote dialogs for one window.
//
// The shell paints Session.Page next to the log and calls Handle for toolbar
// callbacks. Git runs on a worker the shell starts; results come back through
// Apply, which the shell calls from OnWake.
type Session struct {
	Panel       Panel
	Checkout    CheckoutDialog
	NewBranch   NewBranchDialog
	Rename      RenameDialog
	Delete      DeleteDialog
	Push        PushDialog
	Pull        PullDialog
	Stash       StashPanel
	Remotes     RemotesDialog
	Credentials CredentialsDialog
	Toaster     ui.Toaster

	Favorites []string
	Compare   string

	repo     *repo.Repo
	settings config.RepoSettings
	runner   *command.Runner
	snap     repo.Snapshot
	ahead    map[string][2]int

	stashReq        StashRequest
	pendingCheckout string
	pendingPush     bool
	fetchToast      ui.ToastID
	publish         func(UIEvent)
	start           func(func(context.Context))
	refresh         func()
}

// UIEvent is a dialog or toast change a git worker asks the UI thread to apply.
type UIEvent struct {
	Error          string
	ClearError     bool
	Toast          string
	ToastBody      string
	ToastKind      ui.ToastKind
	LoadingID      ui.ToastID
	DoneTitle      string
	DoneBody       string
	DoneKind       ui.ToastKind
	CheckoutBranch string
	CheckoutFiles  []string
	Unmerged       *ops.ErrNotFullyMerged
	PushTargets    []PushTarget
	PushAllowForce bool
	PushResults    []ops.PushNativeResult
	ShowPush       bool
	ShowRejected   bool
	Stashes        []model.Stash
	ShowStash      bool
	UpdateStash    bool
	Refresh        bool
}

// SetRunner installs the shell's worker starter and log refresh.
func (s *Session) SetRunner(start func(func(context.Context)), refresh func()) {
	s.start = start
	s.refresh = refresh
	s.Panel.OnRequest = func(req Request) { s.Handle(req, s.start, s.refresh) }
}
func (s *Session) SetPublisher(publish func(UIEvent)) { s.publish = publish }

// Bind attaches the session to the open repository. Call it when a repo opens.
func (s *Session) Bind(repository *repo.Repo, runner *command.Runner, settings config.RepoSettings, invalid *viva.Invalidator) {
	s.repo = repository
	s.runner = runner
	s.settings = settings
	s.Credentials.SetInvalidator(invalid)
	s.Panel.OnRequest = func(req Request) { s.Handle(req, s.start, s.refresh) }
	if repository != nil {
		s.snap = repo.Snapshot{
			Path: repository.Path, State: repository.State, HEAD: repository.HEAD,
			CurrentBranch: repository.CurrentBranch, Branches: repository.Branches,
			Tags: repository.Tags, Remotes: repository.Remotes, Tracking: repository.Tracking,
		}
		s.Panel.Load(s.snap, s.ahead, s.Favorites)
	}
}

// Repo is the bound repository, or nil.
func (s *Session) Repo() *repo.Repo { return s.repo }

// Page paints the side panel. Dialogs and toasts are painted by Overlays
// so a modal is not clipped by the side column.
func (s *Session) Page(c ui.C) {
	if s.repo == nil {
		return
	}
	s.Panel.Page(c)
}

// Overlays paints dialogs, toasts, and the credentials prompt.
func (s *Session) Overlays(c ui.C) {
	if s.repo == nil {
		s.Credentials.Page(c)
		return
	}
	s.dialogs(c)
	ui.Toasts(c, &s.Toaster)
	s.Credentials.Page(c)
}

// Handle runs a toolbar or context-menu request. start is how the shell
// runs git off the UI thread. refresh asks the shell to reload the log.
func (s *Session) Handle(req Request, start func(func(context.Context)), refresh func()) {
	switch req.Action {
	case ActionCheckout:
		s.pendingCheckout = req.Ref
		s.startCheckout(req.Ref, false, start, refresh)
	case ActionNewBranch:
		s.NewBranch.Show(req.Ref)
	case ActionRename:
		s.Rename.Show(req.Ref)
	case ActionDelete:
		s.Delete.Show(req.Ref)
	case ActionMerge:
		s.runMerge(req.Ref, start, refresh)
	case ActionRebase:
		s.runRebase(req.Ref, start, refresh)
	case ActionCompare:
		s.Compare = req.Ref
	case ActionCheckoutRebase:
		s.checkoutAndRebase(req.Ref, start, refresh)
	case ActionFavorite:
		s.toggleFavorite(req.Ref)
	case ActionPush:
		s.preparePush(start)
	case ActionPull:
		s.Pull.Show(s.defaultRemote(), s.currentBranch(), s.settings.UpdateMethod)
	case ActionFetch:
		s.startFetch(start, refresh)
	case ActionStash:
		s.openStash(start)
	case ActionRemotes:
		s.Remotes.Show(s.snap.Remotes)
	}
}

func (s *Session) dialogs(c ui.C) {
	s.Checkout.Page(c)
	s.NewBranch.Page(c)
	s.Rename.Page(c)
	s.Delete.Page(c)
	s.Push.Page(c)
	s.Pull.Page(c)
	s.stashReq = s.Stash.Page(c)
	if s.Stash.Ready().Action != StashNone {
		s.stashReq = s.Stash.TakeReady()
	}
	s.Remotes.Page(c)
}

// AfterPaint applies dialog confirmations. The shell calls it at the end of
// its page, then starts the matching git work. Returning true means the
// shell should refresh the log after the queued work.
func (s *Session) AfterPaint(start func(func(context.Context)), refresh func()) {
	if s.Checkout.Confirmed {
		s.Checkout.Confirmed = false
		name := s.pendingCheckout
		start(func(ctx context.Context) {
			_, err := ops.Branches{Service: ops.New(s.runner, s.repo.Path)}.CheckoutWithStash(ctx, name, "Checkout")
			s.finishCheckout(err, name)
		})
	}
	if s.NewBranch.Confirmed {
		s.NewBranch.Confirmed = false
		name, startPoint := s.NewBranch.Name, s.NewBranch.StartPoint
		start(func(ctx context.Context) {
			_, err := ops.Branches{Service: ops.New(s.runner, s.repo.Path)}.CheckoutNew(ctx, name, startPoint)
			s.finish(err, "Created "+name)
		})
	}
	if s.Rename.Confirmed {
		s.Rename.Confirmed = false
		old, name := s.Rename.Old, strings.TrimSpace(s.Rename.Name)
		start(func(ctx context.Context) {
			err := ops.Branches{Service: ops.New(s.runner, s.repo.Path)}.Rename(ctx, old, name, false)
			s.finish(err, "Renamed to "+name)
		})
	}
	if s.Delete.Confirmed {
		s.Delete.Confirmed = false
		name, force := s.Delete.Branch, s.Delete.Force
		start(func(ctx context.Context) {
			s.deleteBranch(ctx, name, force, start, refresh)
		})
	}
	if s.Push.Confirmed {
		s.Push.Confirmed = false
		targets, force, tags := append([]PushTarget(nil), s.Push.Targets...), s.Push.ForceLease, s.Push.PushTags
		start(func(ctx context.Context) { s.push(ctx, targets, force, tags, refresh) })
	}
	if s.Push.PullThenPush {
		s.Push.PullThenPush = false
		start(func(ctx context.Context) { s.pullThenPush(ctx, refresh) })
	}
	if s.Pull.Confirmed {
		s.Pull.Confirmed = false
		remote, branch, method := s.Pull.Remote, s.Pull.Branch, s.Pull.Method
		start(func(ctx context.Context) {
			err := ops.NewPuller(s.runner).Pull(ctx, ops.PullParams{
				RepoPath: s.repo.Path, Remote: remote, Branch: branch, Method: MethodOf(method, s.branchRebases()),
			})
			s.finish(err, "Pulled "+remote+"/"+branch)
		})
	}
	if s.stashReq.Action != StashNone {
		req := s.stashReq
		s.stashReq = StashRequest{}
		s.runStash(req, start, refresh)
	}
	if ready := s.Stash.TakeReady(); ready.Action != StashNone {
		s.runStash(ready, start, refresh)
	}
	if s.Remotes.Accepted {
		s.Remotes.Accepted = false
		changes := append([]RemoteChange(nil), s.Remotes.Changes...)
		start(func(ctx context.Context) { s.applyRemotes(ctx, changes, refresh) })
	}
}

func (s *Session) startCheckout(name string, stashed bool, start func(func(context.Context)), refresh func()) {
	start(func(ctx context.Context) {
		branches := ops.Branches{Service: ops.New(s.runner, s.repo.Path)}
		var err error
		if stashed {
			_, err = branches.CheckoutWithStash(ctx, name, "Checkout")
		} else {
			_, err = branches.Checkout(ctx, name)
		}
		var overwritten *command.ErrLocalChangesWouldBeOverwritten
		if errors.As(err, &overwritten) {
			s.emit(UIEvent{CheckoutBranch: name, CheckoutFiles: overwritten.Files, Error: overwritten.Error()})
			return
		}
		s.finishCheckout(err, name)
	})
}

func (s *Session) finishCheckout(err error, name string) {
	var restore *ops.ErrCheckoutRestore
	if errors.As(err, &restore) {
		// Refresh even on a conflicting restore: HEAD has switched and the
		// unmerged index must be visible to the Conflicts screen.
		s.emit(UIEvent{Error: err.Error(), Toast: "Local changes not fully restored", ToastBody: "The stash was kept. Open Conflicts to resolve the remaining changes, or Open stash to inspect the saved changes.", ToastKind: ui.ToastWarning, Refresh: true})
		return
	}
	if err == nil {
		s.emit(UIEvent{ClearError: true, Toast: "Checked out " + name, Refresh: true})
		return
	}
	s.finish(err, "Checked out "+name)
}

func (s *Session) deleteBranch(ctx context.Context, name string, force bool, start func(func(context.Context)), refresh func()) {
	err := ops.Branches{Service: ops.New(s.runner, s.repo.Path)}.Delete(ctx, name, force)
	var unmerged *ops.ErrNotFullyMerged
	if errors.As(err, &unmerged) {
		s.emit(UIEvent{Unmerged: unmerged})
		return
	}
	s.finish(err, "Deleted "+name)
}

func (s *Session) preparePush(start func(func(context.Context))) {
	start(func(ctx context.Context) {
		targets, err := s.outgoing(ctx)
		if err != nil {
			s.emit(UIEvent{Error: err.Error()})
			return
		}
		current := s.currentBranch()
		allow := ForcePushAllowed(current, s.settings.ForcePushProhibitedPatterns)
		s.emit(UIEvent{ShowPush: true, PushTargets: targets, PushAllowForce: allow})
	})
}

func (s *Session) outgoing(ctx context.Context) ([]PushTarget, error) {
	var targets []PushTarget
	for _, ref := range s.snap.Branches {
		if ref.Ref.Kind() != model.KindLocalBranch {
			continue
		}
		name := ref.Ref.Name()
		remote, target := s.upstream(name)
		if remote == "" {
			remote = s.defaultRemote()
			target = name
		}
		spec := remote + "/" + target + ".." + name
		commits, err := s.repo.Log(ctx, repo.LogQuery{Range: spec, MaxCount: 50})
		if err != nil {
			commits, err = s.repo.Log(ctx, repo.LogQuery{Branch: name, MaxCount: 50})
			if err != nil {
				return nil, err
			}
		}
		if len(commits) == 0 {
			continue
		}
		targets = append(targets, PushTarget{
			Branch: name, Remote: remote, Target: target, Selected: name == s.currentBranch(), Commits: commits,
		})
	}
	return targets, nil
}

func (s *Session) push(ctx context.Context, targets []PushTarget, force, tags bool, refresh func()) {
	pusher := ops.NewPusher(s.runner)
	var all []ops.PushNativeResult
	for _, target := range targets {
		if !target.Selected {
			continue
		}
		refspec := target.Branch + ":" + target.Target
		remoteName, _ := s.upstream(target.Branch)
		results, err := pusher.Push(ctx, s.repo.Path, target.Remote, refspec, force, remoteName == "")
		if err != nil && len(results) == 0 {
			s.emit(UIEvent{Error: err.Error()})
			return
		}
		all = append(all, results...)
	}
	if tags {
		results, err := pusher.Push(ctx, s.repo.Path, s.defaultRemote(), "--tags", false, false)
		if err != nil && len(results) == 0 {
			s.emit(UIEvent{Error: err.Error()})
		}
		all = append(all, results...)
	}
	if RejectedNonFF(all) {
		s.emit(UIEvent{ShowRejected: true, PushResults: all})
		return
	}
	s.emit(UIEvent{Toast: "Pushed", ToastBody: fmt.Sprintf("%d refs", len(all)), Refresh: true})
}

func (s *Session) pullThenPush(ctx context.Context, refresh func()) {
	err := ops.NewPuller(s.runner).Pull(ctx, ops.PullParams{
		RepoPath: s.repo.Path,
		Remote:   s.defaultRemote(),
		Branch:   s.currentBranch(),
		Method:   MethodOf(s.settings.UpdateMethod, s.branchRebases()),
	})
	if err != nil {
		s.finish(err, "")
		return
	}
	s.preparePush(func(work func(context.Context)) { work(ctx) })
	if s.Push.Open {
		s.Push.Confirmed = true
		s.Push.Open = false
		targets := append([]PushTarget(nil), s.Push.Targets...)
		s.push(ctx, targets, s.Push.ForceLease, s.Push.PushTags, refresh)
	}
}

func (s *Session) startFetch(start func(func(context.Context)), refresh func()) {
	s.fetchToast = s.Toaster.Show("Fetching", s.defaultRemote(), ui.ToastLoading)
	id := s.fetchToast
	remote := s.defaultRemote()
	start(func(ctx context.Context) {
		err := ops.NewFetcher(s.runner).Fetch(ctx, ops.FetchParams{RepoPath: s.repo.Path, Remotes: []string{remote}})
		if err != nil {
			s.emit(UIEvent{LoadingID: id, DoneTitle: "Fetch failed", DoneBody: err.Error(), DoneKind: ui.ToastError, Error: err.Error()})
			return
		}
		s.emit(UIEvent{LoadingID: id, DoneTitle: "Fetched", DoneBody: remote, DoneKind: ui.ToastSuccess, Refresh: true})
	})
}

func (s *Session) openStash(start func(func(context.Context))) {
	start(func(ctx context.Context) {
		entries, err := s.repo.Stashes(ctx)
		if err != nil {
			s.emit(UIEvent{Error: err.Error()})
			return
		}
		s.emit(UIEvent{ShowStash: true, Stashes: entries})
	})
}

func (s *Session) runStash(req StashRequest, start func(func(context.Context)), refresh func()) {
	start(func(ctx context.Context) {
		st := ops.Stash{Service: ops.New(s.runner, s.repo.Path)}
		var err error
		switch req.Action {
		case StashCreate:
			_, err = st.Save(ctx, req.Options)
		case StashApply:
			err = st.Apply(ctx, req.Ref)
		case StashPop:
			err = st.Pop(ctx, req.Ref)
		case StashDrop:
			err = st.Drop(ctx, req.Ref)
		}
		if err != nil {
			s.finish(err, "")
			return
		}
		entries, listErr := s.repo.Stashes(ctx)
		if listErr != nil {
			s.emit(UIEvent{Error: listErr.Error()})
			return
		}
		// An operation result updates the list, without reopening a dialog
		// the user dismissed while git was running. Publish refresh together
		// with the entries so the refresh cannot invalidate a later event.
		s.emit(UIEvent{UpdateStash: true, Stashes: entries, Toast: "Stash updated", Refresh: true})
	})
}

func (s *Session) applyRemotes(ctx context.Context, changes []RemoteChange, refresh func()) {
	manager := ops.NewRemoteManager(s.runner)
	for _, change := range changes {
		var err error
		switch {
		case change.Add:
			err = manager.AddRemote(ctx, s.repo.Path, change.Name, change.URL)
		case change.Remove:
			err = manager.RemoveRemote(ctx, s.repo.Path, change.Old)
		case change.Rename:
			err = manager.RenameRemote(ctx, s.repo.Path, change.Old, change.Name)
			if err == nil && change.URL != "" {
				err = manager.SetRemoteURL(ctx, s.repo.Path, change.Name, change.URL)
			}
		default:
			if change.URL != "" {
				err = manager.SetRemoteURL(ctx, s.repo.Path, change.Name, change.URL)
			}
		}
		if err != nil {
			s.finish(err, "")
			return
		}
	}
	s.emit(UIEvent{Toast: "Remotes saved", Refresh: true})
}

func (s *Session) runMerge(ref string, start func(func(context.Context)), refresh func()) {
	start(func(ctx context.Context) {
		result := ops.Operations{Dir: s.repo.Path, Git: s.runner}.Merge(ctx, ref, ops.MergeOptions{})
		if result.Outcome == ops.OutcomeConflicts {
			s.emit(UIEvent{Error: "Merge conflicts in " + ref, Refresh: true})
			return
		}
		s.finish(result.Err, "Merged "+ref)
	})
}

func (s *Session) runRebase(ref string, start func(func(context.Context)), refresh func()) {
	start(func(ctx context.Context) {
		_, err := s.runner.Run(ctx, command.Cmd{Dir: s.repo.Path, Subcommand: "rebase", Args: []string{ref}, Write: true})
		s.finish(err, "Rebased onto "+ref)
	})
}

func (s *Session) checkoutAndRebase(ref string, start func(func(context.Context)), refresh func()) {
	local := ref
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		local = ref[i+1:]
	}
	start(func(ctx context.Context) {
		branches := ops.Branches{Service: ops.New(s.runner, s.repo.Path)}
		if _, err := branches.Checkout(ctx, local); err != nil {
			if _, err = branches.CheckoutNew(ctx, local, ref); err != nil {
				s.finish(err, "")
				return
			}
		}
		_, err := s.runner.Run(ctx, command.Cmd{Dir: s.repo.Path, Subcommand: "rebase", Args: []string{ref}, Write: true})
		s.finish(err, "Checked out and rebased "+local)
	})
}

func (s *Session) toggleFavorite(name string) {
	for i, fav := range s.Favorites {
		if fav == name {
			s.Favorites = append(s.Favorites[:i], s.Favorites[i+1:]...)
			s.Panel.Load(s.snap, s.ahead, s.Favorites)
			return
		}
	}
	s.Favorites = append(s.Favorites, name)
	s.Panel.Load(s.snap, s.ahead, s.Favorites)
}

func (s *Session) emit(event UIEvent) {
	if s.publish != nil {
		s.publish(event)
	}
}

// ApplyUI runs on the UI thread, from the shell's OnWake.
func (s *Session) ApplyUI(event UIEvent) {
	if event.ClearError {
		s.Panel.Error = ""
	}
	if event.Error != "" {
		s.Panel.Error = event.Error
		s.Toaster.Show("Git error", event.Error, ui.ToastError)
	}
	if event.Toast != "" {
		kind := event.ToastKind
		if kind == 0 {
			kind = ui.ToastSuccess
		}
		s.Toaster.Show(event.Toast, event.ToastBody, kind)
	}
	if event.DoneTitle != "" {
		s.Toaster.Done(event.LoadingID, event.DoneTitle, event.DoneBody, event.DoneKind)
	}
	if event.CheckoutBranch != "" {
		s.pendingCheckout = event.CheckoutBranch
		s.Checkout.ShowLocalChanges(event.CheckoutBranch, event.CheckoutFiles)
	}
	if event.Unmerged != nil {
		s.Delete.ShowUnmerged(event.Unmerged)
	}
	if event.ShowPush {
		s.Push.Show(event.PushTargets, event.PushAllowForce)
	}
	if event.ShowRejected {
		s.Push.ShowRejected(event.PushResults)
	}
	if event.ShowStash {
		s.Stash.Show(event.Stashes)
	}
	if event.UpdateStash {
		s.Stash.Entries = append(s.Stash.Entries[:0], event.Stashes...)
		s.Stash.Selected = min(s.Stash.Selected, max(len(event.Stashes)-1, 0))
	}
}

func (s *Session) finish(err error, ok string) {
	if err != nil {
		s.emit(UIEvent{Error: err.Error()})
		return
	}
	s.emit(UIEvent{Toast: ok, Refresh: ok != ""})
}

func (s *Session) currentBranch() string { return s.snap.CurrentBranch.Name() }

func (s *Session) defaultRemote() string {
	for _, remote := range s.snap.Remotes {
		if remote.Name == model.OriginName {
			return remote.Name
		}
	}
	if len(s.snap.Remotes) > 0 {
		return s.snap.Remotes[0].Name
	}
	return model.OriginName
}

func (s *Session) upstream(branch string) (remote, target string) {
	for _, track := range s.snap.Tracking {
		if track.Local.Name() == branch {
			return track.Remote.Remote.Name, track.Remote.NameForRemoteOperations
		}
	}
	return "", ""
}

func (s *Session) branchRebases() bool {
	name := s.currentBranch()
	for _, track := range s.snap.Tracking {
		if track.Local.Name() == name {
			return !track.Merge
		}
	}
	return false
}

// SetSnapshot replaces the tree after the shell reloads the repository.
func (s *Session) SetSnapshot(snap repo.Snapshot, ahead map[string][2]int) {
	s.snap = snap
	s.ahead = ahead
	s.Panel.Load(snap, ahead, s.Favorites)
}

// CredentialProvider returns the dialog as the askpass provider.
func (s *Session) CredentialProvider() ops.CredentialProvider { return &s.Credentials }
