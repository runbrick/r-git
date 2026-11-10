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
	uiapp "github.com/runbrick/r-git/internal/ui/app"
)

// version is stamped by the linker with -X main.version=...
// A source build reports "devel".
var version = "devel"

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
	return uiapp.Run(dir, opts.RepoPath)
}

// errHelp is returned by ParseArgs for -h and --help.
var errHelp = errors.New("help")
