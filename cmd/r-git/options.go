package main

import (
	"fmt"
	"strings"
)

// StartOptions is the process startup request. RepoPath empty means no
// repository was named on the command line.
type StartOptions struct {
	RepoPath    string
	ConfigDir   string
	ShowVersion bool
}

func (o StartOptions) usage() string {
	return strings.TrimLeft(`
Usage: r-git [path]
       r-git --config-dir DIR
       r-git --version

  path
    	repository to open
  --config-dir DIR
    	settings directory (default: R_GIT_CONFIG_DIR, or the OS user config dir / r-git)
  --version
    	print the version and exit
`, "\n")
}

// ParseArgs reads command-line arguments into StartOptions.
// -h and --help return errHelp. Unknown flags and extra arguments return
// an error that names the problem.
func ParseArgs(args []string) (StartOptions, error) {
	var opts StartOptions
	var path string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			rest := args[i+1:]
			if len(rest) > 1 || (path != "" && len(rest) > 0) {
				return StartOptions{}, fmt.Errorf("r-git: too many arguments")
			}
			if len(rest) == 1 {
				path = rest[0]
			}
			i = len(args)
		case arg == "-h" || arg == "--help":
			return opts, errHelp
		case arg == "--version" || arg == "-version":
			opts.ShowVersion = true
		case arg == "--config-dir" || arg == "-config-dir":
			if i+1 >= len(args) {
				return StartOptions{}, fmt.Errorf("r-git: --config-dir needs a directory")
			}
			i++
			opts.ConfigDir = args[i]
		case strings.HasPrefix(arg, "--config-dir="):
			opts.ConfigDir = strings.TrimPrefix(arg, "--config-dir=")
			if opts.ConfigDir == "" {
				return StartOptions{}, fmt.Errorf("r-git: --config-dir needs a directory")
			}
		case strings.HasPrefix(arg, "-"):
			return StartOptions{}, fmt.Errorf("r-git: unknown flag %s", arg)
		default:
			if path != "" {
				return StartOptions{}, fmt.Errorf("r-git: too many arguments")
			}
			path = arg
		}
	}
	opts.RepoPath = path
	return opts, nil
}
