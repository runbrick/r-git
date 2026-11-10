package branches

import (
	"context"
	"sync"

	"github.com/runbrick/r-git/internal/git/ops"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// credentialPrompt is one askpass request waiting on the UI thread.
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
// Credentials blocks the askpass goroutine on a channel. The UI thread
// paints the modal and answers it. A cancelled context or a dismissed
// dialog returns an error so git's askpass exits without a password.
type CredentialsDialog struct {
	mu       sync.Mutex
	waiting  *credentialPrompt
	username string
	password string
	shown    bool
	invalid  *viva.Invalidator
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
	d.waiting = prompt
	d.shown = true
	invalid := d.invalid
	d.mu.Unlock()
	if invalid != nil {
		invalid.Invalidate()
	}
	select {
	case <-ctx.Done():
		d.clear(prompt)
		return ops.Credential{}, ctx.Err()
	case answer := <-prompt.result:
		if !answer.ok {
			return ops.Credential{}, context.Canceled
		}
		return answer.cred, nil
	}
}

func (d *CredentialsDialog) clear(prompt *credentialPrompt) {
	d.mu.Lock()
	if d.waiting == prompt {
		d.waiting = nil
		d.shown = false
	}
	d.mu.Unlock()
}

// Page paints the credentials modal when a prompt is waiting.
// Call it once from the shell page, on the UI thread.
func (d *CredentialsDialog) Page(c ui.C) {
	d.mu.Lock()
	prompt := d.waiting
	open := &d.shown
	d.mu.Unlock()
	if prompt == nil || !d.shown {
		return
	}
	title := "Git credentials"
	if prompt.req.URL != "" {
		title = "Git credentials for " + prompt.req.URL
	}
	ui.Modal(c, open, title, func() {
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
		if ui.Button(c, "Cancel").Clicked() {
			d.answer(prompt, credentialAnswer{})
		}
		if ui.Button(c, "OK").Clicked() {
			d.answer(prompt, credentialAnswer{cred: ops.Credential{Username: d.username, Password: d.password}, ok: true})
		}
	})
	if !d.shown {
		d.answer(prompt, credentialAnswer{})
	}
}

func (d *CredentialsDialog) answer(prompt *credentialPrompt, answer credentialAnswer) {
	d.mu.Lock()
	if d.waiting == prompt {
		d.waiting = nil
		d.shown = false
	}
	d.mu.Unlock()
	select {
	case prompt.result <- answer:
	default:
	}
}

// Pending reports whether a prompt is waiting for the user.
func (d *CredentialsDialog) Pending() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.waiting != nil
}
