package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cleanPath cleans up a path and expands ~ to the user's home directory
// if it is present. It then converts the path to an absolute path.
func cleanPath(path string) string {
	cleanedPath := filepath.Clean(path)

	if strings.HasPrefix(cleanedPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting user's home directory: %s", err)
		}
		expandedPath := filepath.Join(homeDir, cleanedPath[2:])
		cleanedPath = expandedPath
	}

	absPath, err := filepath.Abs(cleanedPath)
	if err != nil {
		log.Fatalf("Error converting to absolute path: %s", err)
	}
	return absPath
}

// ExecuteCommandResult represents the result of executing a CLI command.
type ExecuteCommandResult struct {
	Stdout string
	Stderr string
}

// ExecuteCommand executes the given CLI command and streams the output in real time
func ExecuteCommand(command string, args ...string) error {
	var result ExecuteCommandResult

	cmd := exec.Command(command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	outScanner := bufio.NewScanner(stdout)
	errScanner := bufio.NewScanner(stderr)
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		for outScanner.Scan() {
			fmt.Println(outScanner.Text())
			result.Stdout += outScanner.Text() + "\n"
		}
		if err := outScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "error reading stdout:", err)
		}
		close(stdoutDone)
	}()
	go func() {
		for errScanner.Scan() {
			fmt.Println(errScanner.Text())
			result.Stderr += errScanner.Text() + "\n"
		}
		if err := errScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "error reading stderr:", err)
		}
		close(stderrDone)
	}()
	if err := cmd.Wait(); err != nil {
		return err
	}
	<-stdoutDone
	<-stderrDone
	return nil
}
