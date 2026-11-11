package branches

import (
	"context"
	"sync"

	"github.com/runbrick/r-git/internal/git/ops"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// credentialPrompt is one askpass request waiting for the UI thread.
type credentialPrompt struct {
	req    ops.CredentialRequest
	result chan credentialAnswer
}

type credentialAnswer struct {
	cred ops.Credential
	ok   bool
}

// CredentialsDialog implements ops.CredentialProvider.
//
// Credentials blocks the askpass goroutine on a channel. Prompts wait in a
// FIFO queue guarded by mu; several fetches can ask at once and each is
// shown in turn. The modal's open flag is UI-thread state. Askpass code
// only enqueues and removes under mu, then asks the window to repaint.
type CredentialsDialog struct {
	mu      sync.Mutex
	queue   []*credentialPrompt
	invalid *viva.Invalidator

	// open, current, and the fields are UI-thread state. ui.Modal writes
	// open. Askpass goroutines do not touch them.
	open     bool
	current  *credentialPrompt
	seenURL  string
	username string
	password string
}

// SetInvalidator is how a prompt asks the window to repaint. The shell
// sets it to the same Invalidator the window uses.
func (d *CredentialsDialog) SetInvalidator(invalid *viva.Invalidator) {
	d.mu.Lock()
	d.invalid = invalid
	d.mu.Unlock()
}

// Credentials implements ops.CredentialProvider.
func (d *CredentialsDialog) Credentials(ctx context.Context, req ops.CredentialRequest) (ops.Credential, error) {
	prompt := &credentialPrompt{req: req, result: make(chan credentialAnswer, 1)}
	d.mu.Lock()
	d.queue = append(d.queue, prompt)
	invalid := d.invalid
	d.mu.Unlock()
	if invalid != nil {
		invalid.Invalidate()
	}
	select {
	case <-ctx.Done():
		d.remove(prompt)
		if invalid != nil {
			invalid.Invalidate()
		}
		return ops.Credential{}, ctx.Err()
	case answer := <-prompt.result:
		if !answer.ok {
			return ops.Credential{}, context.Canceled
		}
		return answer.cred, nil
	}
}

func (d *CredentialsDialog) remove(prompt *credentialPrompt) {
	d.mu.Lock()
	for i, queued := range d.queue {
		if queued == prompt {
			d.queue = append(d.queue[:i], d.queue[i+1:]...)
			break
		}
	}
	d.mu.Unlock()
}

func (d *CredentialsDialog) head() *credentialPrompt {
	if len(d.queue) == 0 {
		return nil
	}
	return d.queue[0]
}

// Page paints the credentials modal for the prompt at the head of the queue.
// Call it once from the shell page, on the UI thread.
func (d *CredentialsDialog) Page(c ui.C) {
	d.mu.Lock()
	head := d.head()
	d.mu.Unlock()

	if d.current != nil && d.current != head {
		d.password = ""
		if head == nil || head.req.URL != d.current.req.URL {
			d.username = ""
		}
		d.current = nil
		d.open = false
	}
	if head == nil {
		return
	}
	if d.current != head {
		if d.seenURL != head.req.URL {
			d.username = ""
		}
		d.password = ""
		d.seenURL = head.req.URL
		d.current = head
		d.open = true
	}

	prompt := d.current
	title := "Git credentials"
	if prompt.req.URL != "" {
		title = "Git credentials for " + prompt.req.URL
	}
	var response *credentialAnswer
	ui.Modal(c, &d.open, title, func() {
		ui.Column(c).Gap(8).FillWidth().Children(func() {
			if prompt.req.Prompt != "" {
				ui.Text(c, prompt.req.Prompt).FontSize(12).Muted()
			}
			if prompt.req.Kind != ops.CredentialKindPassword {
				ui.TextField(c, &d.username, "Username", "").AutoFocus()
			}
			ui.TextField(c, &d.password, "Password", "").Secure()
		})
	}, func() {
		if ui.Button(c, "Cancel").Outline().Clicked() {
			response = &credentialAnswer{}
			d.open = false
		}
		if ui.Button(c, "OK").Clicked() {
			response = &credentialAnswer{cred: ops.Credential{Username: d.username, Password: d.password}, ok: true}
			d.open = false
		}
	})
	if response != nil || !d.open {
		answer := credentialAnswer{}
		if response != nil {
			answer = *response
		}
		d.finish(prompt, answer)
	}
}

// finish delivers answer, drops the prompt, and forgets the secret.
// The username is kept only when the next prompt is for the same URL.
func (d *CredentialsDialog) finish(prompt *credentialPrompt, answer credentialAnswer) {
	d.password = ""
	d.mu.Lock()
	for i, queued := range d.queue {
		if queued == prompt {
			d.queue = append(d.queue[:i], d.queue[i+1:]...)
			break
		}
	}
	next := d.head()
	invalid := d.invalid
	d.mu.Unlock()
	if next == nil || next.req.URL != prompt.req.URL {
		d.username = ""
	}
	if d.current == prompt {
		d.current = nil
		d.open = false
	}
	select {
	case prompt.result <- answer:
	default:
	}
	if invalid != nil {
		invalid.Invalidate()
	}
}

// Pending reports whether a prompt is waiting for the user.
func (d *CredentialsDialog) Pending() bool {
	return d.PendingCount() > 0
}

// PendingCount is how many prompts are queued, including the one on screen.
func (d *CredentialsDialog) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.queue)
}

// Password is the password field, for tests that check it is cleared.
func (d *CredentialsDialog) Password() string { return d.password }

// Username is the username field, for tests that check it is cleared.
func (d *CredentialsDialog) Username() string { return d.username }
