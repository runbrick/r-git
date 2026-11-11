package main

import (
	"errors"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    StartOptions
		wantErr string
		help    bool
	}{
		{name: "none", args: nil, want: StartOptions{}},
		{name: "path", args: []string{`D:\work\repo`}, want: StartOptions{RepoPath: `D:\work\repo`}},
		{name: "config dir", args: []string{"--config-dir", `D:\cfg`, `.\repo`}, want: StartOptions{RepoPath: `.\repo`, ConfigDir: `D:\cfg`}},
		{name: "config dir equals", args: []string{"--config-dir=" + `D:\cfg`}, want: StartOptions{ConfigDir: `D:\cfg`}},
		{name: "version", args: []string{"--version"}, want: StartOptions{ShowVersion: true}},
		{name: "version short", args: []string{"-version", "repo"}, want: StartOptions{RepoPath: "repo", ShowVersion: true}},
		{name: "end of flags", args: []string{"--", "--weird"}, want: StartOptions{RepoPath: "--weird"}},
		{name: "help", args: []string{"--help"}, help: true},
		{name: "help short", args: []string{"-h"}, help: true},
		{name: "unknown", args: []string{"--nope"}, wantErr: "unknown flag --nope"},
		{name: "missing dir", args: []string{"--config-dir"}, wantErr: "--config-dir needs a directory"},
		{name: "empty dir", args: []string{"--config-dir="}, wantErr: "--config-dir needs a directory"},
		{name: "two paths", args: []string{"a", "b"}, wantErr: "too many arguments"},
		{name: "path then end", args: []string{"a", "--", "b"}, wantErr: "too many arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArgs(tt.args)
			if tt.help {
				if !errors.Is(err, errHelp) {
					t.Fatalf("err = %v, want help", err)
				}
				return
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestUsageMentionsFlags(t *testing.T) {
	text := new(StartOptions).usage()
	for _, needle := range []string{"[path]", "--config-dir", "--version"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("usage missing %q:\n%s", needle, text)
		}
	}
}
