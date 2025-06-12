package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"bytes"
	"fmt"
	"io"
	// "os/exec" // No longer needed directly due to type inference for ExecuteCommand
	"path/filepath"
	"time" // For spinner

	"github.com/briandowns/spinner" // For spinner
	"github.com/spf13/cobra"
)

// MockExecuteCommandInfo holds information about calls to the mock ExecuteCommand
type MockExecuteCommandInfo struct {
	Command string
	Args    []string
	Calls   int
}

var mockExecuteCmdInfo MockExecuteCommandInfo
var mockExecuteCommandShouldError bool
var originalExecuteCommand func(command string, args ...string) error

// mockExecuteCommand replaces the real ExecuteCommand for testing
func mockExecuteCommand(command string, args ...string) error {
	mockExecuteCmdInfo.Command = command
	mockExecuteCmdInfo.Args = args
	mockExecuteCmdInfo.Calls++
	if mockExecuteCommandShouldError {
		return fmt.Errorf("mock ExecuteCommand error")
	}
	// Simulate temp file creation for "docker compose -f <tempfile>" commands
	if command == "docker" && len(args) > 2 && args[0] == "compose" && args[1] == "-f" {
		// args[2] is the temp file path
		// Check if file exists, if not, create it (like createTempComposeFile would)
		// This is a simplification; real createTempComposeFile does more.
		// For these tests, we mostly care about the arguments passed to docker.
		if _, err := os.Stat(args[2]); os.IsNotExist(err) {
			os.WriteFile(args[2], []byte("services: {}"), 0644)
		}
	}
	return nil
}

func setup(t *testing.T) {
	// Reset Viper for this specific test to ensure a clean state.
	viper.Reset()

	// Mock ExecuteCommand
	originalExecuteCommand = ExecuteCommand
	ExecuteCommand = mockExecuteCommand
	mockExecuteCmdInfo = MockExecuteCommandInfo{}
	mockExecuteCommandShouldError = false

	// Save the current global cfgFile value to restore it after the test.
	// This is important for test isolation if TestMain is not used or if
	// tests within this file could affect each other's view of cfgFile.
	previousCfgFileValue := cfgFile
	t.Cleanup(func() {
		cfgFile = previousCfgFileValue
	})

	// Mock configuration
	mockYAMLConfig := `
services:
  service1:
    image: nginx
  service2:
    image: redis
`
	tempCfgFile, err := os.CreateTemp(t.TempDir(), "test_upctl-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	// t.TempDir() automatically cleans up the created temp file and directory.

	if _, err := tempCfgFile.WriteString(mockYAMLConfig); err != nil {
		tempCfgFile.Close()
		t.Fatalf("Failed to write to temp config file: %v", err)
	}
	if err := tempCfgFile.Close(); err != nil {
		t.Fatalf("Failed to close temp config file: %v", err)
	}

	// Set the global cfgFile variable. This is what initConfig (from root.go) will use.
	cfgFile = tempCfgFile.Name()

	// Configure Viper for THIS test's explicit ReadInConfig.
	// This ensures that code within the test itself that uses Viper directly
	// (before any Cobra command execution) sees the correct config.
	viper.SetConfigFile(cfgFile)
	viper.SetConfigType("yaml") // Crucial for Viper to know how to parse the file.
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("setup: viper.ReadInConfig() failed for cfgFile '%s': %v", cfgFile, err)
	}

	// Ensure dockerComposeConfig is populated, as some tested functions might use it directly.
	if err := viper.Unmarshal(&dockerComposeConfig); err != nil {
		t.Fatalf("Failed to unmarshal mock config into dockerComposeConfig: %v", err)
	}
}

func teardown() {
	// Restore ExecuteCommand
	ExecuteCommand = originalExecuteCommand

	// cfgFile is restored by t.Cleanup in setup.
	// Viper is reset by setup at the beginning of the next test.
	// If using TestMain, TestMain would handle final viper.Reset() and cfgFile restoration.

	// Clean up any stray docker-compose-*.yml files (these are not part of t.TempDir)
	files, _ := filepath.Glob("docker-compose-*.yml")
	for _, f := range files {
		os.Remove(f)
	}
}

var originalGlobalCfgFile string // Captured by TestMain

func TestMain(m *testing.M) {
	originalGlobalCfgFile = cfgFile // Save at the very start
	exitCode := m.Run()
	cfgFile = originalGlobalCfgFile // Restore at the very end
	viper.Reset() // Final Viper reset for the package
	os.Exit(exitCode)
}

// Helper to execute cobra command and capture output
func executeCommandCobra(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestRunDockerComposeUp_AllServices(t *testing.T) {
	setup(t)
	defer teardown()

	// rootCmd needs to be accessible or re-created here for testing
	// For simplicity, we'll assume upCmd can be tested via RunE or by constructing a temporary root
	// Let's use the actual rootCmd from the package
	testRootCmd, _ := InitializeTestCmd() // Need a helper to get configured rootCmd

	_, err := executeCommandCobra(testRootCmd, "up", "--all")
	if err != nil {
		t.Fatalf("up --all failed: %v", err)
	}

	if mockExecuteCmdInfo.Calls == 0 {
		t.Errorf("ExecuteCommand was not called for 'up --all'")
	} else {
		expectedArgs := []string{"compose", "-f", mockExecuteCmdInfo.Args[2], "up", "-d"} // temp file path is dynamic
		if mockExecuteCmdInfo.Command != "docker" || !equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgs[:2]) || !equalSlices(mockExecuteCmdInfo.Args[3:], expectedArgs[3:]) {
			t.Errorf("Expected docker compose ... up -d, got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
		// Check that no service name was added
		if len(mockExecuteCmdInfo.Args) > 5 { // docker compose -f <file> up -d (5 elements if no service)
			t.Errorf("Expected no service arguments for --all, got %v", mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeUp_SpecificService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	_, err := executeCommandCobra(testRootCmd, "up", "service1")
	if err != nil {
		t.Fatalf("up service1 failed: %v", err)
	}

	if mockExecuteCmdInfo.Calls == 0 {
		t.Errorf("ExecuteCommand was not called for 'up service1'")
	} else {
		// Expected: docker compose -f <tempfile> up -d service1
		expectedArgsSuffix := []string{"up", "-d", "service1"}
		actualArgsSuffix := mockExecuteCmdInfo.Args[3:] // Skip 'compose', '-f', '<tempfile>'

		if mockExecuteCmdInfo.Command != "docker" || !equalSlices(actualArgsSuffix, expectedArgsSuffix) {
			t.Errorf("Expected docker compose ... up -d service1, got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeUp_NoServiceNoAll(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	output, err := executeCommandCobra(testRootCmd, "up")
	if err == nil {
		t.Fatalf("Expected error for 'up' with no args and no --all, but got none")
	}
	expectedErrorMsg := "you must specify a service name or use the --all flag"
	if !strings.Contains(output, expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, output)
	}
	if mockExecuteCmdInfo.Calls > 0 {
		t.Errorf("ExecuteCommand should not have been called, but was called %d times", mockExecuteCmdInfo.Calls)
	}
}

func TestRunDockerComposeUp_AllAndService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	output, err := executeCommandCobra(testRootCmd, "up", "--all", "service1")
	if err == nil {
		t.Fatalf("Expected error for 'up --all service1', but got none")
	}
	expectedErrorMsg := "cannot specify service names when the --all flag is used"
	if !strings.Contains(output, expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, output)
	}
	if mockExecuteCmdInfo.Calls > 0 {
		t.Errorf("ExecuteCommand should not have been called, but was called %d times", mockExecuteCmdInfo.Calls)
	}
}

// equalSlices checks if two string slices are equal.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// TestMain can be used for setup/teardown if needed, or individual test setup.
// For now, setup/teardown functions are called by each test.

// InitializeTestCmd duplicated from root_test.go (or should be shared)
// This is a placeholder for however rootCmd is configured and made available for tests.
// It needs to register all flags and subcommands as in the main init().
func InitializeTestCmd() (*cobra.Command, error) {
    // Simplified init for testing. Production init is in root.go's init()
    // This is a common challenge in testing Cobra apps.
    // We need a way to get a 'fresh' command tree for each test execution.

    // Re-create the root command and its subcommands for testing to avoid state leakage
    var testRootCmd = &cobra.Command{Use: "upctl"}
    // Add flags and commands similar to how it's done in the actual init() function in root.go

    // upCmd definition would be here or imported/copied
    var testUpCmd = &cobra.Command{
        Use:   "up [service]",
        Short: "Start specified or all services using Docker Compose",
        Long:  `Starts the services defined in your upctl.yaml file using Docker Compose. Equivalent to 'docker compose up -d'. You can optionally specify a single service to start, or use the --all flag to start all services.`,
        Args:  cobra.ArbitraryArgs,
        RunE: func(ccmd *cobra.Command, args []string) error {
            allServices, _ := ccmd.Flags().GetBool("all")
            numArgs := len(args)

            if allServices {
                if numArgs > 0 {
                    return fmt.Errorf("cannot specify service names when the --all flag is used")
                }
            } else {
                if numArgs == 0 {
                    return fmt.Errorf("you must specify a service name or use the --all flag")
                }
                if numArgs > 1 {
                    // This specific error message from root.go was:
                    // "too many arguments, expected 1 service name or --all flag (got %d)"
                    // For this test, a generic one is fine if we are not testing the exact message from RunE here
                    // but rather the call to RunDockerComposeUp.
                    // However, for error case tests, we test the exact error message.
                    return fmt.Errorf("too many arguments, expected 1 service name or --all flag (got %d)", numArgs)

                }
            }
            // In real scenario, RunDockerComposeUp is called.
            // For tests focusing on argument parsing by RunE, we might not call it
            // or call a mock version of it.
            // For these tests, we assume RunDockerComposeUp is called if no error.
            if progress == nil { // from root.go
                 // Initialize progress spinner if it's nil (copied from root.go)
                 // This might need to be properly initialized or stubbed out for tests
                 progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard))
            }
            RunDockerComposeUp(ccmd, args) // This will use the mocked ExecuteCommand
            return nil
        },
    }
    testUpCmd.Flags().BoolP("all", "a", false, "Start all services")

    // downCmd definition (copied and adapted from root.go)
    var testDownCmd = &cobra.Command{
        Use:   "down [service]",
        Short: "Stop Docker Compose services",
        Args:  cobra.ArbitraryArgs,
        RunE: func(ccmd *cobra.Command, args []string) error {
            allServices, _ := ccmd.Flags().GetBool("all")
            numArgs := len(args)
            if allServices {
                if numArgs > 0 { return fmt.Errorf("cannot specify service names when the --all flag is used for 'down'") }
            } else {
                if numArgs == 0 { return fmt.Errorf("you must specify a service name or use the --all flag for 'down'") }
                if numArgs > 1 { return fmt.Errorf("too many arguments to 'down', expected 1 service name or --all flag (got %d)", numArgs) }
            }
            if progress == nil { progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard)) }
            RunDockerComposeDown(ccmd, args) // Will use mocked ExecuteCommand
            return nil
        },
    }
    testDownCmd.Flags().BoolP("all", "a", false, "Stop all services")

    // logsCmd definition (copied and adapted from root.go)
    var testLogsCmd = &cobra.Command{
        Use:   "logs [service]",
        Short: "Show logs for services",
        Args:  cobra.ArbitraryArgs,
        RunE: func(ccmd *cobra.Command, args []string) error {
            allServices, _ := ccmd.Flags().GetBool("all")
            numArgs := len(args)
            if allServices {
                if numArgs > 0 { return fmt.Errorf("cannot specify service names when the --all flag is used for 'logs'") }
            } else {
                if numArgs == 0 { return fmt.Errorf("you must specify a service name or use the --all flag for 'logs'") }
                if numArgs > 1 { return fmt.Errorf("too many arguments to 'logs', expected 1 service name or --all flag (got %d)", numArgs) }
            }
            if progress == nil { progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard)) }
            RunDockerComposeLogs(ccmd, args) // Will use mocked ExecuteCommand
            return nil
        },
    }
    testLogsCmd.Flags().BoolP("all", "a", false, "Get logs for all services")

    testRootCmd.AddCommand(testUpCmd, testDownCmd, testLogsCmd)

    return testRootCmd, nil
}

// This is line 603 in the previous file listing.
// The following lines were missing.
func TestCreateTempComposeFile_NoVersionKey(t *testing.T) {
	viper.Reset() // Ensure clean viper state for this specific test

	// This test should ideally not interact with global cfgFile.
	// If createTempComposeFile indirectly calls initConfig, it might.
	// For now, we assume TestMain handles overall Viper state, and this test
	// just needs its own mock config loaded.

	mockYAMLConfig := `
services:
  web:
    image: nginx
    ports:
      - "80:80"
  db:
    image: postgres
    volumes:
      - db_data:/var/lib/postgresql/data
volumes:
  db_data: {}
networks:
  front-tier: {}
  back-tier: {}
`
	// Load mock config into viper for this test
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(strings.NewReader(mockYAMLConfig))
	if err != nil {
		t.Fatalf("Failed to read mock YAML config for TestCreateTempComposeFile: %v", err)
	}
	// Ensure dockerComposeConfig is populated for createTempComposeFile
	if err := viper.Unmarshal(&dockerComposeConfig); err != nil {
		t.Fatalf("Failed to unmarshal mock config into dockerComposeConfig for TestCreateTempComposeFile: %v", err)
	}

	// Call the function to be tested
	tempFilePath, err := createTempComposeFile()
	if err != nil {
		t.Fatalf("createTempComposeFile() returned an error: %v", err)
	}
	defer os.Remove(tempFilePath) // Clean up the temporary file

	// Read the content of the generated temporary file
	generatedYAMLBytes, errR := os.ReadFile(tempFilePath)
	if errR != nil {
		t.Fatalf("Failed to read temporary compose file '%s': %v", tempFilePath, errR)
	}

	// Unmarshal the generated YAML to check its structure
	var generatedContent map[string]interface{}
	err = yaml.Unmarshal(generatedYAMLBytes, &generatedContent)
	if err != nil {
		t.Fatalf("Failed to unmarshal generated YAML content: %v", err)
	}

	// Assert that the top-level "version" key does NOT exist
	if _, exists := generatedContent["version"]; exists {
		t.Errorf("Expected 'version' key to be absent in generated docker-compose.yml, but it was found.")
	}

	// Assert that 'services' key is present
	servicesField, servicesExists := generatedContent["services"]
	if !servicesExists {
		t.Errorf("'services' key not found in generated docker-compose.yml")
	} else {
		// Basic check for content (can be more detailed)
		servicesMap, ok := servicesField.(map[string]interface{})
		if !ok {
			t.Errorf("'services' field is not a map")
		} else if _, webExists := servicesMap["web"]; !webExists {
			t.Errorf("'services.web' not found in generated YAML")
		}
	}

	// Assert that 'volumes' key is present (if in mock)
	_, volumesExists := generatedContent["volumes"]
	if !volumesExists {
		t.Errorf("'volumes' key not found in generated docker-compose.yml")
	}

	// Assert that 'networks' key is present (if in mock)
	_, networksExists := generatedContent["networks"]
	if !networksExists {
		t.Errorf("'networks' key not found in generated docker-compose.yml")
	}
}

// Tests for downCmd
func TestRunDockerComposeDown_AllServices(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	_, err := executeCommandCobra(testRootCmd, "down", "--all")
	if err != nil {
		t.Fatalf("down --all failed: %v", err)
	}
	if mockExecuteCmdInfo.Calls == 0 {
		t.Error("ExecuteCommand was not called for 'down --all'")
	} else {
		// Expected: docker compose -f <tempfile> down
		// Args[2] is the temp file path, which is dynamic.
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"down"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:4], expectedArgsSuffix) && // Args[3] should be "down"
			len(mockExecuteCmdInfo.Args) == 4) { // docker compose -f <file> down
			t.Errorf("Expected 'docker compose -f <file> down', got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeDown_SpecificService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	_, err := executeCommandCobra(testRootCmd, "down", "service1")
	if err != nil {
		t.Fatalf("down service1 failed: %v", err)
	}
	if mockExecuteCmdInfo.Calls == 0 {
		t.Error("ExecuteCommand was not called for 'down service1'")
	} else {
		// Expected: docker compose -f <tempfile> down service1
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"down", "service1"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:5], expectedArgsSuffix) && // Args[3] is "down", Args[4] is "service1"
			len(mockExecuteCmdInfo.Args) == 5) { // docker compose -f <file> down service1
			t.Errorf("Expected 'docker compose -f <file> down service1', got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeDown_NoServiceNoAll(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()
	output, err := executeCommandCobra(testRootCmd, "down")
	if err == nil {
		t.Fatal("Expected error for 'down' with no args and no --all, but got none")
	}
	expectedErrorMsg := "you must specify a service name or use the --all flag for 'down'"
	if !strings.Contains(output, expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, output)
	}
	if mockExecuteCmdInfo.Calls > 0 {
		t.Errorf("ExecuteCommand should not have been called, but was called %d times", mockExecuteCmdInfo.Calls)
	}
}

func TestRunDockerComposeDown_AllAndService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()
	output, err := executeCommandCobra(testRootCmd, "down", "--all", "service1")
	if err == nil {
		t.Fatal("Expected error for 'down --all service1', but got none")
	}
	expectedErrorMsg := "cannot specify service names when the --all flag is used for 'down'"
	if !strings.Contains(output, expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, output)
	}
	if mockExecuteCmdInfo.Calls > 0 {
		t.Errorf("ExecuteCommand should not have been called, but was called %d times", mockExecuteCmdInfo.Calls)
	}
}

// Tests for logsCmd
func TestRunDockerComposeLogs_AllServices(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	_, err := executeCommandCobra(testRootCmd, "logs", "--all")
	if err != nil {
		t.Fatalf("logs --all failed: %v", err)
	}
	if mockExecuteCmdInfo.Calls == 0 {
		t.Error("ExecuteCommand was not called for 'logs --all'")
	} else {
		// Expected: docker compose -f <tempfile> logs --follow
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"logs", "--follow"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:5], expectedArgsSuffix) && // Args[3] is "logs", Args[4] is "--follow"
			len(mockExecuteCmdInfo.Args) == 5) { // docker compose -f <file> logs --follow
			t.Errorf("Expected 'docker compose -f <file> logs --follow', got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeLogs_SpecificService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()

	_, err := executeCommandCobra(testRootCmd, "logs", "service1")
	if err != nil {
		t.Fatalf("logs service1 failed: %v", err)
	}
	if mockExecuteCmdInfo.Calls == 0 {
		t.Error("ExecuteCommand was not called for 'logs service1'")
	} else {
		// Expected: docker compose -f <tempfile> logs --follow service1
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"logs", "--follow", "service1"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:6], expectedArgsSuffix) && // Args[3] is "logs", Args[4] is "--follow", Args[5] is "service1"
			len(mockExecuteCmdInfo.Args) == 6) { // docker compose -f <file> logs --follow service1
			t.Errorf("Expected 'docker compose -f <file> logs --follow service1', got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeLogs_NoServiceNoAll(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()
	output, err := executeCommandCobra(testRootCmd, "logs")
	if err == nil {
		t.Fatal("Expected error for 'logs' with no args and no --all, but got none")
	}
	expectedErrorMsg := "you must specify a service name or use the --all flag for 'logs'"
	if !strings.Contains(output, expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, output)
	}
	if mockExecuteCmdInfo.Calls > 0 {
		t.Errorf("ExecuteCommand should not have been called, but was called %d times", mockExecuteCmdInfo.Calls)
	}
}

func TestRunDockerComposeLogs_AllAndService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd()
	output, err := executeCommandCobra(testRootCmd, "logs", "--all", "service1")
	if err == nil {
		t.Fatal("Expected error for 'logs --all service1', but got none")
	}
	expectedErrorMsg := "cannot specify service names when the --all flag is used for 'logs'"
	if !strings.Contains(output, expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, output)
	}
	if mockExecuteCmdInfo.Calls > 0 {
		t.Errorf("ExecuteCommand should not have been called, but was called %d times", mockExecuteCmdInfo.Calls)
	}
}
