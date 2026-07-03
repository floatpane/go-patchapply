package patchapply

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GeneratePatch runs `git format-patch --stdout` in repoDir for the given
// commit range and returns the raw format-patch email bytes.
//
// commitRange can be any revision range accepted by git (e.g. "HEAD~1",
// "HEAD~3..HEAD", "origin/main..HEAD", a single commit hash, etc.).
// Pass "HEAD" to format the latest commit.
//
// The function shells out to git; the working directory must be a git
// repository. It does not modify the repository in any way.
func GeneratePatch(repoDir, commitRange string) ([]byte, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("repoDir is required")
	}
	if commitRange == "" {
		return nil, fmt.Errorf("commitRange is required")
	}

	// Validate that git is available.
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found in PATH: %w", err)
	}

	args := []string{"format-patch", "--stdout"}
	// Handle ranges that contain ".." or multiple revs by passing as-is.
	// For a single commit, add "-1" to format just that commit.
	if !strings.Contains(commitRange, "..") && !strings.Contains(commitRange, " ") {
		args = append(args, "-1", commitRange)
	} else {
		args = append(args, commitRange)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git format-patch failed: %w", err)
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("git format-patch produced no output for range %q", commitRange)
	}
	return stdout.Bytes(), nil
}

// GeneratePatchSeries runs `git format-patch --stdout` for a range and returns
// the raw mbox bytes containing all patches in the series.
//
// Unlike GeneratePatch, this always treats the input as a range and does not
// add the "-1" flag, so multi-commit ranges produce a single mbox with all
// patches in order.
func GeneratePatchSeries(repoDir, commitRange string) ([]byte, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("repoDir is required")
	}
	if commitRange == "" {
		return nil, fmt.Errorf("commitRange is required")
	}

	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found in PATH: %w", err)
	}

	cmd := exec.Command("git", "format-patch", "--stdout", commitRange)
	cmd.Dir = repoDir
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git format-patch failed: %w", err)
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("git format-patch produced no output for range %q", commitRange)
	}
	return stdout.Bytes(), nil
}
