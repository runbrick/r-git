package commit

import "github.com/runbrick/r-git/internal/i18n"

// UserMessage retains English for callers without a window locale.
func UserMessage(err error) string { return i18n.En.Error(err) }

// UserMessageFor localizes typed errors and preserves Git stderr and user content.
func UserMessageFor(messages *i18n.Messages, err error) string { return messages.Error(err) }
