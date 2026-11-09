package testutil

import (
	"os"
	"strings"
	"testing"
)

func TestRepoHelpers(t *testing.T) {
	repo := NewRepo(t)
	repo.WriteFile("nested/readme.txt", "fixture\n")
	hash := repo.Commit("add fixture")
	if len(hash) != 40 {
		t.Fatalf("commit hash length = %d, want 40", len(hash))
	}
	if got := strings.TrimSpace(repo.Git("log", "-1", "--format=%s")); got != "add fixture" {
		t.Fatalf("commit subject = %q, want %q", got, "add fixture")
	}
	content, err := os.ReadFile(repo.Dir + string(os.PathSeparator) + "nested" + string(os.PathSeparator) + "readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "fixture\n" {
		t.Fatalf("file content = %q, want fixture with LF", content)
	}
}
