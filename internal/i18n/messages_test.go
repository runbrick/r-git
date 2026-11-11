package i18n

import (
	"errors"
	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
	"reflect"
	"testing"
	"time"
)

func TestBundlesAreComplete(t *testing.T) {
	for _, messages := range []*Messages{&ZhCN, &En} {
		value := reflect.ValueOf(*messages)
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.IsZero() {
				t.Errorf("empty message %s", value.Type().Field(i).Name)
			}
		}
	}
}
func TestChineseDates(t *testing.T) {
	when := time.Date(2026, 6, 16, 23, 31, 0, 0, time.UTC)
	if got := ZhCN.ShortDate(when); got != "2026/6/16 23:31" {
		t.Fatal(got)
	}
	if got := ZhCN.RelativeDate(when, when.Add(5*time.Minute)); got != "5 分钟前" {
		t.Fatal(got)
	}
}
func TestErrorsPreserveGitOutput(t *testing.T) {
	if got := ZhCN.Error(ops.ErrEmptyCommit); got != ZhCN.NothingToCommitStageAChangeOrIncludeAFile {
		t.Fatal(got)
	}
	raw := "fatal: 用户 supplied path.txt"
	if got := ZhCN.Error(&command.ExitError{Stderr: raw}); got != raw {
		t.Fatal(got)
	}
	if got := ZhCN.Error(errors.New("user message")); got != "user message" {
		t.Fatal(got)
	}
}
func TestLanguageOverride(t *testing.T) {
	for _, tc := range []struct{ override, saved, want string }{{"", "zh-CN", "zh-CN"}, {"", "en", "en"}, {"en", "zh-CN", "en"}, {"zh-CN", "en", "zh-CN"}, {"invalid", "en", "en"}} {
		t.Setenv("R_GIT_LANG", tc.override)
		if got := StartupLanguage(tc.saved); got != tc.want {
			t.Errorf("%+v got %s", tc, got)
		}
	}
}
