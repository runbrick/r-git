package ops

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
)

type RebaseEntry struct{ Action, Commit, Subject string }

func ParseTodo(todo []byte) ([]RebaseEntry, error) {
	var entries []RebaseEntry
	s := bufio.NewScanner(bytes.NewReader(todo))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("%w: %q", ErrInvalidTodo, line)
		}
		subject := ""
		if len(parts) == 3 {
			subject = parts[2]
		}
		entries = append(entries, RebaseEntry{Action: parts[0], Commit: parts[1], Subject: subject})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func ApplyTodoModel(entries []RebaseEntry) ([]RebaseEntry, error) {
	valid := map[string]bool{"pick": true, "p": true, "reword": true, "r": true, "edit": true, "e": true, "squash": true, "s": true, "fixup": true, "f": true, "drop": true, "d": true, "exec": true, "x": true, "break": true, "b": true, "label": true, "reset": true, "merge": true}
	for i, e := range entries {
		if !valid[e.Action] {
			return nil, fmt.Errorf("%w: unknown action %q", ErrInvalidTodo, e.Action)
		}
		if (e.Action == "fixup" || e.Action == "f" || e.Action == "squash" || e.Action == "s") && i == 0 {
			return nil, fmt.Errorf("%w: %s cannot be the first entry", ErrInvalidTodo, e.Action)
		}
	}
	return append([]RebaseEntry(nil), entries...), nil
}

func WriteTodo(path string, entries []RebaseEntry) error {
	valid, err := ApplyTodoModel(entries)
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, e := range valid {
		fmt.Fprintf(&b, "%s %s", e.Action, e.Commit)
		if e.Subject != "" {
			fmt.Fprintf(&b, " %s", e.Subject)
		}
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0600)
}

// HandleSequenceEditor implements the file protocol used by Git's sequence editor.
func HandleSequenceEditor(_ context.Context, path string, edit func([]RebaseEntry) ([]RebaseEntry, error)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	entries, err := ParseTodo(data)
	if err != nil {
		return err
	}
	if edit != nil {
		entries, err = edit(entries)
		if err != nil {
			return err
		}
	}
	return WriteTodo(path, entries)
}
