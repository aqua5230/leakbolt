//go:build darwin

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const graphicalSessionProbeScript = `use framework "AppKit"
set frontApp to current application's NSWorkspace's sharedWorkspace()'s frontmostApplication()
if frontApp is missing value then error "no graphical session"
return "ok"`

func platformNativeDialogsSupported() bool {
	return true
}

func platformShowBlockedCommitDialog(findings []stagedFinding) (string, error) {
	if _, err := runAppleScript(2*time.Second, graphicalSessionProbeScript); err != nil {
		return "", err
	}
	return runAppleScript(125*time.Second, blockedCommitDialogScript(findings))
}

func runAppleScript(timeout time.Duration, script string) (string, error) {
	path, err := exec.LookPath("osascript")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "-e", script).Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("osascript: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
