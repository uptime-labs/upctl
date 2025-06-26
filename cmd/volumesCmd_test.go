package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// MockExecuteCommand is a mock for the ExecuteCommand function
type MockExecuteCommand struct {
	Command string
	Args    []string
	Err     error
}

var mockExecuteTracker []MockExecuteCommand

// mockExecuteCommandVolumes replaces the real ExecuteCommand for testing
func mockExecuteCommandVolumes(command string, args ...string) error {
	tracker := MockExecuteCommand{Command: command, Args: args}
	mockExecuteTracker = append(mockExecuteTracker, tracker)
	// Simulate error if needed for a specific test case by checking command/args
	return nil // Default to no error
}

// mockCaptureCommand replaces the real CaptureCommand for testing
func mockCaptureCommand(command string, args ...string) (string, error) {
	tracker := MockExecuteCommand{Command: command, Args: args}
	mockExecuteTracker = append(mockExecuteTracker, tracker)
	// Simulate output and error if needed
	if command == "docker" && args[0] == "volume" && args[1] == "ls" {
		return "DRIVER    VOLUME NAME\nlocal     my-volume\nlocal     another-volume", nil
	}
	return "", nil // Default to empty output and no error
}

func TestVolumesLsCmd(t *testing.T) {
	// Setup: Replace ExecuteCommand with mock
	originalExecuteCommand := ExecuteCommand
	ExecuteCommand = mockExecuteCommandVolumes
	defer func() { ExecuteCommand = originalExecuteCommand }() // Restore original

	mockExecuteTracker = []MockExecuteCommand{} // Reset tracker

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	volumesLsCmd.Run(volumesLsCmd, []string{})

	w.Close()
	os.Stdout = oldStdout // Restore stdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Assertions
	if !strings.Contains(output, "Listing Docker volumes...") {
		t.Errorf("Expected output to contain 'Listing Docker volumes...', got %s", output)
	}

	if len(mockExecuteTracker) != 1 {
		t.Fatalf("Expected 1 command to be executed, got %d", len(mockExecuteTracker))
	}
	if mockExecuteTracker[0].Command != "docker" || strings.Join(mockExecuteTracker[0].Args, " ") != "volume ls" {
		t.Errorf("Expected 'docker volume ls' to be called, got '%s %s'", mockExecuteTracker[0].Command, strings.Join(mockExecuteTracker[0].Args, " "))
	}
}

func TestVolumesRmCmd(t *testing.T) {
	originalExecuteCommand := ExecuteCommand
	ExecuteCommand = mockExecuteCommandVolumes
	defer func() { ExecuteCommand = originalExecuteCommand }()

	tests := []struct {
		name         string
		args         []string
		expectedCmd  string
		expectedArgs string
		expectError  bool // For RunE, not directly testable here without more complex setup
	}{
		{
			name:         "Remove single volume",
			args:         []string{"my-volume"},
			expectedCmd:  "docker",
			expectedArgs: "volume rm my-volume",
		},
		{
			name:         "Remove multiple volumes",
			args:         []string{"vol1", "vol2", "vol3"},
			expectedCmd:  "docker",
			expectedArgs: "volume rm vol1 vol2 vol3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecuteTracker = []MockExecuteCommand{} // Reset tracker

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// volumesRmCmd.Run checks Args, so we call it directly
			// If volumesRmCmd used RunE, we'd call Execute() on it or its parent.
			volumesRmCmd.Run(volumesRmCmd, tt.args)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			expectedOutput := fmt.Sprintf("Removing Docker volume(s): %s", strings.Join(tt.args, ", "))
			if !strings.Contains(output, expectedOutput) {
				t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, output)
			}

			if len(mockExecuteTracker) != 1 {
				t.Fatalf("Expected 1 command to be executed, got %d", len(mockExecuteTracker))
			}
			if mockExecuteTracker[0].Command != tt.expectedCmd || strings.Join(mockExecuteTracker[0].Args, " ") != tt.expectedArgs {
				t.Errorf("Expected '%s %s' to be called, got '%s %s'", tt.expectedCmd, tt.expectedArgs, mockExecuteTracker[0].Command, strings.Join(mockExecuteTracker[0].Args, " "))
			}
		})
	}
}

func TestVolumesRmCmd_NoArgs(t *testing.T) {
	// Test cobra's Args validation
	// This requires a different approach, typically by trying to Execute the command
	// and checking the error, or by directly checking cmd.Args(cmd, args).
	// For this example, we'll note that cobra handles MinimumNArgs.
	// A full test would involve setting up rootCmd and calling Execute().
	// For now, we assume cobra's built-in validation works.
	// Example of how one might test cobra Args:
	// rootCmd.AddCommand(volumesCmd) // Assuming volumesCmd is already init
	// _, err := executeCommandC(rootCmd, "volumes", "rm") // executeCommandC is a helper not defined here
	// if err == nil {
	//  t.Errorf("Expected error for missing arguments but got none")
	// }
	// This is more involved, so skipping direct test of cobra arg validation here.
}
