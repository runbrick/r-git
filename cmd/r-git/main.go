// Command r-git opens a repository in the log view.
//
//	r-git [path]
//	r-git --config-dir DIR
//	r-git --version
//
// Settings are loaded once at startup and owned by the UI goroutine.
// Save is explicit: nothing in this process writes settings on a timer.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/runbrick/r-git/internal/config"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// version is stamped by the linker with -X main.version=...
// A source build reports "devel".
var version = "devel"

type model struct {
	opts   StartOptions
	app    config.AppSettings
	repo   config.RepoSettings
	config string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Children(func() {
		// TODO: honor AppSettings.ShowLogInEditor when the log view lands.
		// The empty state from PR0 stays until that screen replaces it.
		ui.Button(c, "Open a repository")
	})
}

func main() {
	opts, err := ParseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprint(os.Stdout, opts.usage())
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if opts.ShowVersion {
		fmt.Println(version)
		os.Exit(0)
	}
	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

func run(opts StartOptions) error {
	dir := opts.ConfigDir
	if dir == "" {
		var err error
		dir, err = config.Dir()
		if err != nil {
			return err
		}
	}
	appSettings, err := config.LoadApp(dir)
	if err != nil {
		return err
	}
	var repoSettings config.RepoSettings
	if opts.RepoPath != "" {
		repoSettings, err = config.LoadRepo(dir, opts.RepoPath)
		if err != nil {
			return err
		}
	} else {
		repoSettings = config.DefaultRepoSettings()
	}
	m := &model{
		opts:   opts,
		app:    appSettings,
		repo:   repoSettings,
		config: dir,
	}
	return ui.Run(app.Config{Title: "r-git"}, m.page)
}

// errHelp is returned by ParseArgs for -h and --help.
var errHelp = errors.New("help")
