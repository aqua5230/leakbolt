//go:build !darwin

package main

func platformNativeDialogsSupported() bool {
	return false
}

func platformShowBlockedCommitDialog([]stagedFinding) (string, error) {
	return "", nil
}
