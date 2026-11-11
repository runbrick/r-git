package config

import (
	"os"
	"testing"
)

func TestThemeMigrationAndRoundTrip(t *testing.T) {
	for _, tc := range []struct{ name, data, theme string }{
		{"unversioned", `{"showLogInEditor":false,"maxRecentProjects":7}`, "dark"},
		{"old v1", `{"version":1,"data":{"showLogInEditor":false,"maxRecentProjects":7}}`, "dark"},
		{"empty theme", `{"version":1,"data":{"theme":"","showLogInEditor":false,"maxRecentProjects":7}}`, "dark"},
		{"invalid theme", `{"version":1,"data":{"theme":"system","showLogInEditor":false,"maxRecentProjects":7}}`, "dark"},
		{"light", `{"version":1,"data":{"theme":"light","showLogInEditor":false,"maxRecentProjects":7}}`, "light"},
		{"unversioned light", `{"theme":"light","showLogInEditor":false,"maxRecentProjects":7}`, "light"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(AppPath(dir), []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			s, err := LoadApp(dir)
			if err != nil {
				t.Fatal(err)
			}
			if s.Theme != tc.theme || s.ShowLogInEditor || s.MaxRecentProjects != 7 {
				t.Fatalf("migration lost appearance or existing settings: %+v", s)
			}
			s.Theme = "light"
			if err := SaveApp(dir, s); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadApp(dir)
			if err != nil || loaded.Theme != "light" {
				t.Fatalf("saved light appearance: %+v, %v", loaded, err)
			}
		})
	}
}
