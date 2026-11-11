package config

import (
	"os"
	"testing"
)

func TestLanguageDefaultsAndMigrates(t *testing.T) {
	for _, body := range []string{"", `{"showLogInEditor":true}`, `{"version":1,"data":{"theme":"light"}}`, `{"version":1,"data":{"language":"invalid"}}`} {
		dir := t.TempDir()
		if body != "" {
			if err := os.WriteFile(AppPath(dir), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		settings, err := LoadApp(dir)
		if err != nil || settings.Language != "zh-CN" {
			t.Fatalf("%s: %+v %v", body, settings, err)
		}
		settings.Language = "en"
		if err := SaveApp(dir, settings); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadApp(dir)
		if err != nil || loaded.Language != "en" {
			t.Fatalf("%+v %v", loaded, err)
		}
	}
}
