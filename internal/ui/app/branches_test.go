package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/testutil"
	branchui "github.com/runbrick/r-git/internal/ui/branches"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestBranchFlowsOnBareRemote(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("initial")
	mainBranch := strings.TrimSpace(g.Git("branch", "--show-current"))
	bare := filepath.Join(filepath.Dir(g.Dir), "remote.git")
	git(t, g, "init", "--bare", bare)
	g.Git("remote", "add", "origin", bare)
	g.Git("push", "-u", "origin", mainBranch)

	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading && hasLabel(driver, mainBranch+" (current)") })

	// Create a branch from the current one and check it out.
	chooseBranch(t, driver, mainBranch+" (current)")
	clickLeftmost(t, driver, "Branch actions")
	clickControl(t, driver, "New branch from")
	waitFor(t, driver, func() bool { return m.branches.NewBranch.Open })
	clickControl(t, driver, "Branch name")
	if err := driver.Type("topic"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Create branch")
	waitFor(t, driver, func() bool { return strings.TrimSpace(g.Git("branch", "--show-current")) == "topic" })

	// Delete an unmerged branch asks for confirmation, then force-deletes.
	g.WriteFile("topic.txt", "topic\n")
	g.Commit("topic commit")
	g.Git("checkout", "-q", mainBranch)
	m.refresh()
	waitFor(t, driver, func() bool { return hasLabel(driver, "topic") && strings.Contains(current(g), mainBranch) })
	chooseBranch(t, driver, "topic")
	clickLeftmost(t, driver, "Branch actions")
	clickControl(t, driver, "Delete")
	waitFor(t, driver, func() bool { return m.branches.Delete.Open })
	clickControl(t, driver, "Delete")
	waitFor(t, driver, func() bool { return m.branches.Delete.Open && m.branches.Delete.Unmerged != nil })
	clickControl(t, driver, "Force delete")
	waitFor(t, driver, func() bool { return !strings.Contains(g.Git("branch"), "topic") })

	// Push a new branch to the bare remote.
	g.Git("checkout", "-q", "-b", "pushed")
	g.WriteFile("pushed.txt", "pushed\n")
	g.Commit("to push")
	m.refresh()
	waitFor(t, driver, func() bool { return hasLabel(driver, "pushed (current)") })
	clickRightmost(t, driver, "Push")
	waitFor(t, driver, func() bool { return m.branches.Push.Open && len(m.branches.Push.Targets) > 0 })
	clickControl(t, driver, "Push commits")
	waitFor(t, driver, func() bool {
		out := gitOutput(t, g, "--git-dir", bare, "branch")
		return strings.Contains(out, "pushed")
	})

	// A non-fast-forward push is rejected and offers Pull then push.
	divergeBare(t, g, bare, mainBranch)
	g.Git("checkout", "-q", mainBranch)
	g.WriteFile("local.txt", "local\n")
	g.Commit("local only")
	m.refresh()
	waitFor(t, driver, func() bool { return hasLabel(driver, mainBranch+" (current)") })
	clickRightmost(t, driver, "Push")
	waitFor(t, driver, func() bool { return m.branches.Push.Open })
	clickControl(t, driver, "Push commits")
	waitFor(t, driver, func() bool { return m.branches.Push.ShowResults && branchui.RejectedNonFF(m.branches.Push.Results) })
	if !hasLabel(driver, "Pull then push") {
		t.Fatal("rejected push did not offer Pull then push")
	}

	// Pull with rebase.
	clickControl(t, driver, "Close")
	clickRightmost(t, driver, "Pull")
	waitFor(t, driver, func() bool { return m.branches.Pull.Open })
	clickControl(t, driver, "Rebase")
	if !m.branches.Pull.Open {
		t.Fatalf("pull dialog closed before confirm, method %q", m.branches.Pull.Method)
	}
	clickControl(t, driver, "Pull now")
	waitFor(t, driver, func() bool {
		out := g.Git("log", "--oneline")
		if strings.Contains(out, "local only") && strings.Contains(out, "remote only") {
			return true
		}
		if m.branches.Panel.Error != "" {
			t.Fatalf("pull error: %s\nlog:\n%s", m.branches.Panel.Error, out)
		}
		return false
	})
}

func TestStashAndPop(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("initial")
	g.WriteFile("base.txt", "wip\n")
	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
	clickControl(t, driver, "Open stash")
	waitFor(t, driver, func() bool { return m.branches.Stash.Open })
	clickControl(t, driver, "Message")
	if err := driver.Type("keep this"); err != nil {
		t.Fatalf("type: %v open=%v", err, m.branches.Stash.Open)
	}
	m.branches.Stash.SubmitCreate()
	m.branches.AfterPaint(m.startGit, m.refresh)
	waitFor(t, driver, func() bool {
		body, err := os.ReadFile(filepath.Join(g.Dir, "base.txt"))
		if err == nil && string(body) == "base\n" && len(m.branches.Stash.Entries) > 0 {
			return true
		}
		if m.branches.Panel.Error != "" {
			t.Fatalf("stash error: %s", m.branches.Panel.Error)
		}
		return false
	})
	clickControl(t, driver, "Pop")
	waitFor(t, driver, func() bool { return m.branches.Stash.Open == false || hasLabel(driver, "Pop stash") })
	clickControl(t, driver, "Pop")
	waitFor(t, driver, func() bool {
		body, err := os.ReadFile(filepath.Join(g.Dir, "base.txt"))
		if err == nil && string(body) == "wip\n" {
			return true
		}
		if m.branches.Panel.Error != "" {
			t.Fatalf("pop error: %s", m.branches.Panel.Error)
		}
		return false
	})
}

func TestCheckoutLocalChangesOffersStash(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("initial")
	g.Git("checkout", "-q", "-b", "other")
	g.WriteFile("base.txt", "other\n")
	g.Commit("other")
	mainBranch := firstBranch(g)
	g.Git("checkout", "-q", mainBranch)
	g.WriteFile("base.txt", "dirty\n")

	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && hasLabel(driver, "other") })
	chooseBranch(t, driver, "other")
	clickLeftmost(t, driver, "Branch actions")
	clickControl(t, driver, "Checkout")
	waitFor(t, driver, func() bool { return m.branches.Checkout.Open })
	if !hasLabel(driver, "base.txt") {
		t.Fatal("stash offer did not list the file from the typed error")
	}
	clickControl(t, driver, "Stash and checkout")
	waitFor(t, driver, func() bool { return strings.TrimSpace(g.Git("branch", "--show-current")) == "other" })
}

func TestCredentialsDialogBlocksProvider(t *testing.T) {
	m, driver := openDriver(t, "")
	m.branches.Credentials.SetInvalidator(&m.invalidator)
	done := make(chan ops.Credential, 1)
	failed := make(chan error, 1)
	go func() {
		cred, err := m.branches.Credentials.Credentials(context.Background(), ops.CredentialRequest{
			Prompt: "Username for 'https://example.com'",
			URL:    "https://example.com",
			Kind:   ops.CredentialKindUsername,
		})
		if err != nil {
			failed <- err
			return
		}
		done <- cred
	}()
	waitFor(t, driver, func() bool { return m.branches.Credentials.Pending() && hasLabel(driver, "Username") })
	clickControl(t, driver, "Username")
	if err := driver.Type("ada"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Password")
	if err := driver.Type("secret"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "OK")
	select {
	case cred := <-done:
		if cred.Username != "ada" || cred.Password != "secret" {
			t.Fatalf("credential = %+v", cred)
		}
	case err := <-failed:
		t.Fatal(err)
	}
}

type credentialResult struct {
	cred ops.Credential
	err  error
}

func TestCredentialsQueueAnswersInOrder(t *testing.T) {
	m, driver := openDriver(t, "")
	m.branches.Credentials.SetInvalidator(&m.invalidator)
	first := make(chan credentialResult, 1)
	second := make(chan credentialResult, 1)
	go func() {
		cred, err := m.branches.Credentials.Credentials(context.Background(), ops.CredentialRequest{
			Prompt: "Username for 'https://one.example'",
			URL:    "https://one.example",
			Kind:   ops.CredentialKindUsername,
		})
		first <- credentialResult{cred, err}
	}()
	waitFor(t, driver, func() bool { return hasLabel(driver, "https://one.example") })
	go func() {
		cred, err := m.branches.Credentials.Credentials(context.Background(), ops.CredentialRequest{
			Prompt: "Username for 'https://two.example'",
			URL:    "https://two.example",
			Kind:   ops.CredentialKindUsername,
		})
		second <- credentialResult{cred, err}
	}()
	waitFor(t, driver, func() bool { return m.branches.Credentials.PendingCount() == 2 })
	clickControl(t, driver, "Username")
	if err := driver.Type("one"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Password")
	if err := driver.Type("secret-one"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "OK")
	select {
	case got := <-first:
		if got.err != nil || got.cred.Username != "one" || got.cred.Password != "secret-one" {
			t.Fatalf("first = %+v", got)
		}
	case <-second:
		t.Fatal("second prompt was answered before the first")
	}
	waitFor(t, driver, func() bool { return hasLabel(driver, "https://two.example") })
	if m.branches.Credentials.Password() != "" {
		t.Fatal("password was still filled when the second prompt opened")
	}
	if m.branches.Credentials.Username() != "" {
		t.Fatal("username from a different URL was still filled")
	}
	clickControl(t, driver, "Username")
	if err := driver.Type("two"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Password")
	if err := driver.Type("secret-two"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "OK")
	got := waitCredential(t, second)
	if got.err != nil || got.cred.Username != "two" || got.cred.Password != "secret-two" {
		t.Fatalf("second = %+v", got)
	}
}

func TestCredentialsCancelClosesAndShowsNext(t *testing.T) {
	m, driver := openDriver(t, "")
	m.branches.Credentials.SetInvalidator(&m.invalidator)
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	second := make(chan ops.Credential, 1)
	go func() {
		_, err := m.branches.Credentials.Credentials(ctx, ops.CredentialRequest{
			Prompt: "Username for 'https://gone.example'",
			URL:    "https://gone.example",
			Kind:   ops.CredentialKindUsername,
		})
		first <- err
	}()
	waitFor(t, driver, func() bool { return hasLabel(driver, "https://gone.example") })
	go func() {
		cred, err := m.branches.Credentials.Credentials(context.Background(), ops.CredentialRequest{
			Prompt: "Username for 'https://next.example'",
			URL:    "https://next.example",
			Kind:   ops.CredentialKindUsername,
		})
		if err != nil {
			t.Errorf("second prompt: %v", err)
			return
		}
		second <- cred
	}()
	waitFor(t, driver, func() bool { return m.branches.Credentials.PendingCount() == 2 })
	clickControl(t, driver, "Password")
	if err := driver.Type("leftover"); err != nil {
		t.Fatal(err)
	}
	cancel()
	waitFor(t, driver, func() bool {
		return hasLabel(driver, "https://next.example") && !hasLabel(driver, "https://gone.example")
	})
	if m.branches.Credentials.Password() != "" {
		t.Fatal("cancelled prompt left its password in the next dialog")
	}
	select {
	case err := <-first:
		if err == nil {
			t.Fatal("cancelled prompt returned a credential")
		}
	default:
		t.Fatal("cancelled prompt is still blocked")
	}
	clickControl(t, driver, "Username")
	if err := driver.Type("next"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Password")
	if err := driver.Type("fresh"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "OK")
	select {
	case cred := <-second:
		if cred.Username != "next" || cred.Password != "fresh" {
			t.Fatalf("next = %+v", cred)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("next prompt was not answered")
	}
}

func waitCredential(t *testing.T, ch <-chan credentialResult) credentialResult {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a credential answer")
	}
	return credentialResult{}
}

func chooseBranch(t *testing.T, driver *viva.Driver, label string) {
	t.Helper()
	clickLeftmost(t, driver, label)
}

func clickLeftmost(t *testing.T, driver *viva.Driver, label string) {
	t.Helper()
	hit, ok := findHit(driver, label, true)
	if !ok {
		t.Fatalf("no control %q", label)
	}
	if err := driver.Click(hit.Bounds.X+4, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
}

func clickRightmost(t *testing.T, driver *viva.Driver, label string) {
	t.Helper()
	hit, ok := findHit(driver, label, false)
	if !ok {
		t.Fatalf("no control %q among %v", label, labelValues(driver))
	}
	if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
}

func rightClickLabel(t *testing.T, driver *viva.Driver, label string) {
	t.Helper()
	for _, item := range driver.Snapshot().Labels {
		if item.Value == label {
			if err := driver.ClickButton(item.Bounds.X+4, item.Bounds.Y+item.Bounds.Height/2, gfx.PointerButtonSecondary); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no label %q among %v", label, labelValues(driver))
}

func findHit(driver *viva.Driver, label string, leftmost bool) (gfx.Hit, bool) {
	var found gfx.Hit
	ok := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != label {
			continue
		}
		if !ok || (leftmost && hit.Bounds.X < found.Bounds.X) || (!leftmost && hit.Bounds.X >= found.Bounds.X) {
			found = hit
			ok = true
		}
	}
	return found, ok
}

func current(g *testutil.Repo) string {
	return strings.TrimSpace(g.Git("branch", "--show-current"))
}

func firstBranch(g *testutil.Repo) string {
	out := g.Git("branch", "--list")
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if line != "" && line != "other" {
			return line
		}
	}
	return "master"
}

func divergeBare(t *testing.T, g *testutil.Repo, bare, branch string) {
	t.Helper()
	clone := filepath.Join(filepath.Dir(g.Dir), "clone")
	git(t, g, "clone", bare, clone)
	gitIn(t, g, clone, "config", "user.name", "Test User")
	gitIn(t, g, clone, "config", "user.email", "test@example.invalid")
	write := filepath.Join(clone, "remote.txt")
	if err := os.WriteFile(write, []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, g, clone, "add", "--all")
	gitIn(t, g, clone, "commit", "-q", "-m", "remote only")
	gitIn(t, g, clone, "push", "origin", branch)
}

func git(t *testing.T, g *testutil.Repo, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.Dir(g.Dir)
	cmd.Env = gitEnv(g)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitIn(t *testing.T, g *testutil.Repo, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(g)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOutput(t *testing.T, g *testutil.Repo, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = gitEnv(g)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func gitEnv(g *testutil.Repo) []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.Getenv("GIT_CONFIG_GLOBAL"),
		"GIT_CONFIG_SYSTEM="+os.Getenv("GIT_CONFIG_SYSTEM"),
	)
}
