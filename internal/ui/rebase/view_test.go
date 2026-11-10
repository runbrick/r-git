package rebase

import (
	"errors"
	"testing"

	"github.com/runbrick/r-git/internal/git/ops"
)

func TestValidateTodo(t *testing.T) {
	for _, tt := range []struct {
		name    string
		entries []ops.RebaseEntry
		valid   bool
	}{
		{"squash", []ops.RebaseEntry{{Action: "pick", Commit: "a"}, {Action: "squash", Commit: "b"}}, true},
		{"first fixup", []ops.RebaseEntry{{Action: "fixup", Commit: "a"}}, false},
		{"drop before squash", []ops.RebaseEntry{{Action: "drop", Commit: "a"}, {Action: "squash", Commit: "b"}}, false},
		{"todo injection", []ops.RebaseEntry{{Action: "reword", Subject: "message\ndrop abc"}}, false},
		{"multiple reword", []ops.RebaseEntry{{Action: "reword", Subject: "a"}, {Action: "reword", Subject: "b"}}, false},
		{"reword", []ops.RebaseEntry{{Action: "reword", Subject: "new subject"}}, true},
		{"unsupported", []ops.RebaseEntry{{Action: "exec", Commit: "whoami"}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.entries)
			if (err == nil) != tt.valid {
				t.Fatalf("error %v valid %v", err, tt.valid)
			}
			if err != nil && !errors.Is(err, ops.ErrInvalidTodo) {
				t.Fatal(err)
			}
		})
	}
}

func TestMovePreservesActionAndReword(t *testing.T) {
	v := View{Entries: []ops.RebaseEntry{{Action: "pick", Commit: "a"}, {Action: "reword", Commit: "b", Subject: "changed"}}}
	v.Move(1, -1)
	if v.Entries[0].Commit != "b" || v.Entries[0].Subject != "changed" || v.Entries[0].Action != "reword" {
		t.Fatal(v.Entries)
	}
	v.Move(0, -1)
	if v.Entries[0].Commit != "b" {
		t.Fatal(v.Entries)
	}
}
