//go:build !windows

package command

import "os/exec"

func configureProcess(*exec.Cmd) {}
