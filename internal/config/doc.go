// Package config stores r-git settings outside the user's repository.
//
// Files live under os.UserConfigDir()/r-git unless R_GIT_CONFIG_DIR (or
// --config-dir) overrides that directory. Nothing in this package writes
// into a git working tree. Settings are plain data: the UI goroutine owns
// them and calls Save explicitly. There is no autosave timer.
package config
