package app

import (
	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/testutil"
	viva "github.com/vivaui/viva/app"
	"testing"
)

func TestDefaultChineseAndRuntimeEnglish(t *testing.T) {
	t.Setenv("R_GIT_LANG", "")
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("User content stays unchanged")
	g.WriteFile("notes.txt", "new\n")
	m, err := New(t.TempDir(), g.Dir)
	if err != nil {
		t.Fatal(err)
	}
	d, err := viva.NewDriver(m.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close(); m.stopWorkers() })
	waitFor(t, d, func() bool {
		return m.repo != nil && !m.log.Loading && len(m.log.Details) > 0 && hasLabel(d, "notes.txt")
	})
	for _, label := range []string{"提交", "提交变更", "提交说明", "设置", "分支", "User content stays unchanged"} {
		if !hasLabel(d, label) {
			t.Errorf("default UI missing %q", label)
		}
	}
	if m.appConfig.Language != "zh-CN" {
		t.Fatal("Chinese not default")
	}
	clickControl(t, d, "设置")
	clickControl(t, d, "语言")
	clickControl(t, d, "English")
	clickControl(t, d, "OK")
	for _, label := range []string{"Commit changes", "Commit message", "Settings", "Branches", "User content stays unchanged"} {
		if !hasLabel(d, label) {
			t.Errorf("English UI missing %q", label)
		}
	}
	saved, err := config.LoadApp(m.configDir)
	if err != nil || saved.Language != "en" {
		t.Fatalf("language not saved: %+v %v", saved, err)
	}
	reopened, err := New(m.configDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Text().Commit != "Commit" {
		t.Fatal("saved language not loaded")
	}
	clickControl(t, d, "Settings")
	clickControl(t, d, "Language")
	clickControl(t, d, "简体中文")
	clickControl(t, d, "确定")
	if !hasLabel(d, "提交变更") {
		t.Fatal("Chinese not restored")
	}
}
