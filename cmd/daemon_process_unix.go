//go:build !windows

package cmd

import "os/exec"

func configureDaemonProcess(cmd *exec.Cmd) {}
