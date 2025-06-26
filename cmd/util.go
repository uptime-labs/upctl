package cmd

import (
	"bufio"
	"bytes"
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

	// check backward and forward paths for Windows
	if strings.HasPrefix(cleanedPath, "~/") || strings.HasPrefix(cleanedPath, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting user's home directory: %s", err)
		}
		expandedPath := filepath.Join(homeDir, cleanedPath[2:])
		cleanedPath = expandedPath
	}

	if strings.HasPrefix(cleanedPath, "/tmp") || strings.HasPrefix(cleanedPath, "\\tmp") {
		expandedPath := filepath.Join(os.TempDir(), cleanedPath[2:])
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
var ExecuteCommand = func(command string, args ...string) error {
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
			progress.Restart()
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
			progress.Restart()
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

// CaptureCommand executes the given CLI command and returns its stdout and an error if any.
// Stderr is printed to the console.
var CaptureCommand = func(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf // Capture stderr as well

	err := cmd.Run() // Use Run instead of Start/Wait for simpler capture

	// Print stderr to console, as ExecuteCommand does, but we are capturing stdout
	if stderrBuf.Len() > 0 {
		fmt.Fprintln(os.Stderr, stderrBuf.String())
	}

	if err != nil {
		// Include stderr in the error message if the command failed
		return stdoutBuf.String(), fmt.Errorf("command failed: %s\nStderr: %s", err, stderrBuf.String())
	}

	return stdoutBuf.String(), nil
}

// tshAwsEcrLogin
func tshAwsEcrLogin() (string, error) {
	cmd := exec.Command("tsh", "aws", "--app", dockerConfig.AWSApp, "ecr", "get-login-password", "--region", "eu-west-1")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error executing command: %v, stderr: %s", err, errBuf.String())
	}

	scanner := bufio.NewScanner(&outBuf)
	for scanner.Scan() {
		return scanner.Text(), nil
	}
	return "", fmt.Errorf("no password found")
}

func contains(elements []string, element string) bool {
	for _, n := range elements {
		if n == element {
			return true
		}
	}
	return false
}
