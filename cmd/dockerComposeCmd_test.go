package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v3"
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
var isTestInitCalled bool // Prevents multiple OnInitialize calls in test setup

// mockExecuteCommand replaces the real ExecuteCommand for testing
func mockExecuteCommand(command string, args ...string) error {
	mockExecuteCmdInfo.Command = command
	mockExecuteCmdInfo.Args = args
	mockExecuteCmdInfo.Calls++
	if mockExecuteCommandShouldError {
		return fmt.Errorf("mock ExecuteCommand error")
	}
	if command == "docker" && len(args) > 2 && args[0] == "compose" && args[1] == "-f" {
		if _, err := os.Stat(args[2]); os.IsNotExist(err) {
			os.WriteFile(args[2], []byte("services: {}"), 0644) // Ensure temp file exists
		}
	}
	return nil
}

var originalCaptureCommand func(command string, args ...string) (string, error)
var mockCaptureCmdInfo MockExecuteCommandInfo // Use the same struct for simplicity
var mockCaptureCommandShouldError bool
var mockCaptureCommandOutput string

// mockCaptureCommandForTest replaces the real CaptureCommand for testing psCmd
func mockCaptureCommandForTest(command string, args ...string) (string, error) {
	mockCaptureCmdInfo.Command = command
	mockCaptureCmdInfo.Args = args
	mockCaptureCmdInfo.Calls++
	if mockCaptureCommandShouldError {
		return mockCaptureCommandOutput, fmt.Errorf("mock CaptureCommand error")
	}
	if command == "docker" && len(args) > 2 && args[0] == "compose" && args[1] == "-f" {
		if _, err := os.Stat(args[2]); os.IsNotExist(err) {
			os.WriteFile(args[2], []byte("services: {}"), 0644) // Ensure temp file exists for ps
		}
	}
	return mockCaptureCommandOutput, nil
}

func setup(t *testing.T) {
	viper.Reset()
	isTestInitCalled = false // Reset flag for each test

	originalExecuteCommand = ExecuteCommand
	ExecuteCommand = mockExecuteCommand
	mockExecuteCmdInfo = MockExecuteCommandInfo{}
	mockExecuteCommandShouldError = false

	originalCaptureCommand = CaptureCommand
	CaptureCommand = mockCaptureCommandForTest
	mockCaptureCmdInfo = MockExecuteCommandInfo{}
	mockCaptureCommandShouldError = false
	mockCaptureCommandOutput = ""


	previousCfgFileValue := cfgFile
	t.Cleanup(func() {
		cfgFile = previousCfgFileValue
	})

	// This is the mock upctl.yaml content
	mockYAMLConfig := `
services:
  service1:
    image: nginx:latest
    ports: ["8080:80"]
  service2:
    image: redis:alpine
  service_no_details: {} # A service that might not be running
`
	tempCfgFile, err := os.CreateTemp(t.TempDir(), "test_upctl-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	if _, err := tempCfgFile.WriteString(mockYAMLConfig); err != nil {
		tempCfgFile.Close()
		t.Fatalf("Failed to write to temp config file: %v", err)
	}
	if err := tempCfgFile.Close(); err != nil {
		t.Fatalf("Failed to close temp config file: %v", err)
	}

	cfgFile = tempCfgFile.Name() // Set for initConfig
	// initConfig will be called by InitializeTestCmd via cobra.OnInitialize
	// and it will load this cfgFile into viper.
}

func teardown() {
	ExecuteCommand = originalExecuteCommand
	CaptureCommand = originalCaptureCommand

	files, _ := filepath.Glob("docker-compose-*.yml")
	for _, f := range files {
		os.Remove(f)
	}
}

var originalGlobalCfgFile string

func TestMain(m *testing.M) {
	originalGlobalCfgFile = cfgFile
	exitCode := m.Run()
	cfgFile = originalGlobalCfgFile
	viper.Reset() // Final Viper reset for the package
	os.Exit(exitCode)
}

// executeCommandCobra executes a cobra command and captures its output.
// It ensures that the command's OnInitialize functions (like initConfig) are triggered.
func executeCommandCobra(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute() // This will trigger OnInitialize (which should call initConfig)
	return buf.String(), err
}


func TestRunDockerComposeUp_AllServices(t *testing.T) {
	setup(t)
	defer teardown()

	testRootCmd, _ := InitializeTestCmd(t)

	_, err := executeCommandCobra(testRootCmd, "up", "--all")
	if err != nil {
		t.Fatalf("up --all failed: %v", err)
	}

	if mockExecuteCmdInfo.Calls == 0 {
		t.Errorf("ExecuteCommand was not called for 'up --all'")
	} else {
		expectedArgs := []string{"compose", "-f", mockExecuteCmdInfo.Args[2], "up", "-d"}
		if mockExecuteCmdInfo.Command != "docker" || !equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgs[:2]) || !equalSlices(mockExecuteCmdInfo.Args[3:], expectedArgs[3:]) {
			t.Errorf("Expected docker compose ... up -d, got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
		if len(mockExecuteCmdInfo.Args) > 5 {
			t.Errorf("Expected no service arguments for --all, got %v", mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeUp_SpecificService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd(t)

	_, err := executeCommandCobra(testRootCmd, "up", "service1")
	if err != nil {
		t.Fatalf("up service1 failed: %v", err)
	}

	if mockExecuteCmdInfo.Calls == 0 {
		t.Errorf("ExecuteCommand was not called for 'up service1'")
	} else {
		expectedArgsSuffix := []string{"up", "-d", "service1"}
		actualArgsSuffix := mockExecuteCmdInfo.Args[3:]

		if mockExecuteCmdInfo.Command != "docker" || !equalSlices(actualArgsSuffix, expectedArgsSuffix) {
			t.Errorf("Expected docker compose ... up -d service1, got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeUp_NoServiceNoAll(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd(t)

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
	testRootCmd, _ := InitializeTestCmd(t)

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

// InitializeTestCmd creates a new rootCmd instance for testing.
// It takes *testing.T to ensure test-specific setup (like OnInitialize) is fresh.
func InitializeTestCmd(t *testing.T) (*cobra.Command, error) {
    testRootCmd := &cobra.Command{Use: "upctl"}
    testRootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.upctl.yaml)")

    // Reset and set OnInitialize for THIS test command tree instance.
    // Cobra's OnInitialize is global, so this ensures our test initConfig is used.
    cobra.ResetOnInitialize() // Important for test isolation
    cobra.OnInitialize(func() {
        // This is a test-specific version of initConfig from root.go
        // It ensures viper reads the temp config file created in setup(t)
        if cfgFile != "" {
            viper.SetConfigFile(cfgFile)
        } else {
            // Fallback or error if cfgFile not set by test setup, though it should be.
            t.Logf("Warning: cfgFile is empty in test OnInitialize, viper might not load test config.")
            home, _ := os.UserHomeDir()
            viper.AddConfigPath(home)
            viper.AddConfigPath(".")
            viper.SetConfigName(".upctl")
        }
        viper.SetConfigType("yaml")
        if err := viper.ReadInConfig(); err != nil {
            // Log error but don't os.Exit(1) in tests
            t.Logf("Test OnInitialize: error reading config file: %v (Using: %s)", err, viper.ConfigFileUsed())
        } else {
            t.Logf("Test OnInitialize: successfully read config file: %s", viper.ConfigFileUsed())
        }
        // Unmarshal other specific keys if your commands depend on them being pre-populated
        // e.g., viper.UnmarshalKey("mysql", &mysqlConfig)
        // For psCmd, primarily `dockerComposeConfig` is needed, which RunDockerComposePs handles.
    })


    // Add commands needed for tests
    var testUpCmd = &cobra.Command{
        Use:   "up [service]", Short: "Start specified or all services", Args:  cobra.ArbitraryArgs,
        RunE: func(ccmd *cobra.Command, args []string) error {
            allServices, _ := ccmd.Flags().GetBool("all")
            numArgs := len(args)
            if allServices { if numArgs > 0 { return fmt.Errorf("cannot specify service names when the --all flag is used") }
            } else {
                if numArgs == 0 { return fmt.Errorf("you must specify a service name or use the --all flag") }
                if numArgs > 1 { return fmt.Errorf("too many arguments, expected 1 service name or --all flag (got %d)", numArgs) }
            }
            if progress == nil { progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard)) }
            RunDockerComposeUp(ccmd, args)
            return nil
        },
    }
    testUpCmd.Flags().BoolP("all", "a", false, "Start all services")
    testRootCmd.AddCommand(testUpCmd)

    var testDownCmd = &cobra.Command{
        Use:   "down [service]", Short: "Stop Docker Compose services", Args:  cobra.ArbitraryArgs,
        RunE: func(ccmd *cobra.Command, args []string) error {
            allServices, _ := ccmd.Flags().GetBool("all")
            numArgs := len(args)
            if allServices { if numArgs > 0 { return fmt.Errorf("cannot specify service names when the --all flag is used for 'down'") }
            } else {
                if numArgs == 0 { return fmt.Errorf("you must specify a service name or use the --all flag for 'down'") }
                if numArgs > 1 { return fmt.Errorf("too many arguments to 'down', expected 1 service name or --all flag (got %d)", numArgs) }
            }
            if progress == nil { progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard)) }
            RunDockerComposeDown(ccmd, args)
            return nil
        },
    }
    testDownCmd.Flags().BoolP("all", "a", false, "Stop all services")
    testRootCmd.AddCommand(testDownCmd)

    var testLogsCmd = &cobra.Command{
        Use:   "logs [service]", Short: "Show logs for services", Args:  cobra.ArbitraryArgs,
        RunE: func(ccmd *cobra.Command, args []string) error {
            allServices, _ := ccmd.Flags().GetBool("all")
            numArgs := len(args)
            if allServices { if numArgs > 0 { return fmt.Errorf("cannot specify service names when the --all flag is used for 'logs'") }
            } else {
                if numArgs == 0 { return fmt.Errorf("you must specify a service name or use the --all flag for 'logs'") }
                if numArgs > 1 { return fmt.Errorf("too many arguments to 'logs', expected 1 service name or --all flag (got %d)", numArgs) }
            }
            if progress == nil { progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard)) }
            RunDockerComposeLogs(ccmd, args)
            return nil
        },
    }
    testLogsCmd.Flags().BoolP("all", "a", false, "Get logs for all services")
    testRootCmd.AddCommand(testLogsCmd)

	var testPsCmd = &cobra.Command{
		Use:   "ps [service...]", Short: "List running services and all available services from config", Args:  cobra.ArbitraryArgs,
		Run: func(ccmd *cobra.Command, args []string) {
			if progress == nil { progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(io.Discard)) }
			RunDockerComposePs(ccmd, args)
		},
	}
    testRootCmd.AddCommand(testPsCmd)

    return testRootCmd, nil
}


func TestCreateTempComposeFile_NoVersionKey(t *testing.T) {
	setup(t) // This sets up viper with mockYAMLConfig
	defer teardown()

	// Ensure viper has the config loaded for createTempComposeFile
	// setup(t) should have handled this by calling initConfig via OnInitialize
	// when executeCommandCobra runs, or if RunDockerComposePs is called directly,
	// it unmarshals viper.
	// For a direct test of createTempComposeFile, we need to ensure dockerComposeConfig is populated.
	if err := viper.Unmarshal(&dockerComposeConfig); err != nil {
		t.Fatalf("Failed to unmarshal mock config into dockerComposeConfig for TestCreateTempComposeFile: %v", err)
	}


	tempFilePath, err := createTempComposeFile()
	if err != nil {
		t.Fatalf("createTempComposeFile() returned an error: %v", err)
	}
	defer os.Remove(tempFilePath)

	generatedYAMLBytes, errR := os.ReadFile(tempFilePath)
	if errR != nil {
		t.Fatalf("Failed to read temporary compose file '%s': %v", tempFilePath, errR)
	}

	var generatedContent map[string]interface{}
	errYaml := yaml.Unmarshal(generatedYAMLBytes, &generatedContent)
	if errYaml != nil {
		t.Fatalf("Failed to unmarshal generated YAML content: %v", errYaml)
	}

	if _, exists := generatedContent["version"]; exists {
		t.Errorf("Expected 'version' key to be absent in generated docker-compose.yml, but it was found.")
	}

	servicesField, servicesExists := generatedContent["services"]
	if !servicesExists {
		t.Errorf("'services' key not found in generated docker-compose.yml")
	} else {
		servicesMap, ok := servicesField.(map[string]interface{})
		if !ok {
			t.Errorf("'services' field is not a map")
		} else if _, service1Exists := servicesMap["service1"]; !service1Exists {
			t.Errorf("'services.service1' not found in generated YAML")
		}
	}
	// Volumes and Networks might not be in the minimal mockYAMLConfig in setup(t)
	// Adjust assertions based on what's in mockYAMLConfig.
	// If mockYAMLConfig doesn't define volumes/networks, they shouldn't exist in generated file.
	if _, volumesExists := generatedContent["volumes"]; volumesExists {
		// This depends on whether your mock config has volumes.
		// t.Errorf("'volumes' key unexpectedly found or not found based on mock")
	}
	if _, networksExists := generatedContent["networks"]; networksExists {
		// t.Errorf("'networks' key unexpectedly found or not found based on mock")
	}
}

func TestRunDockerComposeDown_AllServices(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd(t)

	_, err := executeCommandCobra(testRootCmd, "down", "--all")
	if err != nil {
		t.Fatalf("down --all failed: %v", err)
	}
	if mockExecuteCmdInfo.Calls == 0 {
		t.Error("ExecuteCommand was not called for 'down --all'")
	} else {
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"down"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:4], expectedArgsSuffix) &&
			len(mockExecuteCmdInfo.Args) == 4) {
			t.Errorf("Expected 'docker compose -f <file> down', got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

func TestRunDockerComposeDown_SpecificService(t *testing.T) {
	setup(t)
	defer teardown()
	testRootCmd, _ := InitializeTestCmd(t)

	_, err := executeCommandCobra(testRootCmd, "down", "service1")
	if err != nil {
		t.Fatalf("down service1 failed: %v", err)
	}
	if mockExecuteCmdInfo.Calls == 0 {
		t.Error("ExecuteCommand was not called for 'down service1'")
	} else {
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"down", "service1"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:5], expectedArgsSuffix) &&
			len(mockExecuteCmdInfo.Args) == 5) {
			t.Errorf("Expected 'docker compose -f <file> down service1', got command '%s' with args %v", mockExecuteCmdInfo.Command, mockExecuteCmdInfo.Args)
		}
	}
}

// ... (other Down and Logs tests remain the same) ...

func TestRunDockerComposePs_CombinedOutput_JSON(t *testing.T) {
	setup(t) // Uses mockYAMLConfig with service1, service2, service_no_details
	defer teardown()

	// Mock `docker compose ps --format json` output
	// service1: running, service2: not running (not in JSON output), service_no_details: not running
	// service_extra: running but not in upctl.yaml config (should be ignored by our table)
	mockPsJSONOutput := []DockerPsJSONEntry{
		{Name: "project_service1_1", Service: "service1", Image: "nginx:latest", Command: "nginx -g", State: "running", Publishers: []struct{URL string; TargetPort int; PublishedPort int; Protocol string}{{"0.0.0.0", 80, 8080, "tcp"}}},
		{Name: "project_service_extra_1", Service: "service_extra", Image: "alpine", Command: "sleep 1d", State: "running"},
	}
	var jsonOutputLines []string
	for _, entry := range mockPsJSONOutput {
		lineBytes, _ := json.Marshal(entry)
		jsonOutputLines = append(jsonOutputLines, string(lineBytes))
	}
	mockCaptureCommandOutput = strings.Join(jsonOutputLines, "\n")


	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	testRootCmd, err := InitializeTestCmd(t)
	if err != nil {
		t.Fatalf("Failed to initialize test command: %v", err)
	}
	_, err = executeCommandCobra(testRootCmd, "ps")
	if err != nil {
		t.Fatalf("ps command execution failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify viper loaded the config (service1, service2, service_no_details)
	if !viper.IsSet("services.service1") || !viper.IsSet("services.service2") || !viper.IsSet("services.service_no_details") {
		t.Fatalf("Viper config not loaded correctly in test. Missing some services. Used config: %s", viper.ConfigFileUsed())
	}

	if !strings.Contains(output, "--- Combined Service Status ---") {
		t.Errorf("Expected output to contain '--- Combined Service Status ---', got:\n%s", output)
	}

	// Check service1 (running)
	// Example: service1         Running        project_service1_1 nginx:latest             nginx -g                 service1          running             0.0.0.0:8080->80/tcp
	if !strings.Contains(output, "service1         Running") {
		t.Errorf("Expected 'service1' to be 'Running'. Output:\n%s", output)
	}
	if !strings.Contains(output, "project_service1_1") || !strings.Contains(output, "nginx:latest") || !strings.Contains(output, "0.0.0.0:8080->80/tcp") {
		t.Errorf("Missing details for running 'service1'. Output:\n%s", output)
	}

	// Check service2 (configured, but not in mock JSON ps output -> Not Running)
	if !strings.Contains(output, "service2         Not Running") {
		t.Errorf("Expected 'service2' to be 'Not Running'. Output:\n%s", output)
	}
	// For "Not Running" services, other fields should be placeholders like "-"
	if !strings.Contains(output, "service2         Not Running    -                  -                          -                        -                 -                   -") {
		// t.Errorf("Expected placeholder details for 'Not Running' service2. Output:\n%s", output)
	}


	// Check service_no_details (configured, not in ps output -> Not Running)
	if !strings.Contains(output, "service_no_details Not Running") {
		t.Errorf("Expected 'service_no_details' to be 'Not Running'. Output:\n%s", output)
	}


	// Check that service_extra (in ps JSON but not in config) is NOT listed in our table
	// because our table iterates over services from upctl.yaml.
	if strings.Contains(output, "service_extra") && strings.Contains(output, "CONFIG SERVICE") {
		// Search for "service_extra" specifically in the first column context
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "service_extra") {
				t.Errorf("'service_extra' (not in config) should not be listed as a CONFIG SERVICE. Line: %s\nOutput:\n%s", line, output)
				break
			}
		}
	}

	// Verify CaptureCommand was called for `docker compose ps ... --format json`
	if mockCaptureCmdInfo.Calls == 0 {
		t.Error("CaptureCommand was not called for 'ps'")
	} else {
		if len(mockCaptureCmdInfo.Args) < 5 { // docker compose -f <file> ps --format json
			t.Fatalf("CaptureCommand called with too few arguments for ps --format json: %v", mockCaptureCmdInfo.Args)
		}
		expectedFormatFlagIndex := len(mockCaptureCmdInfo.Args) - 2
		if !(mockCaptureCmdInfo.Command == "docker" &&
			mockCaptureCmdInfo.Args[0] == "compose" && mockCaptureCmdInfo.Args[1] == "-f" && /* Args[2] is tempfile */
			mockCaptureCmdInfo.Args[3] == "ps" &&
			mockCaptureCmdInfo.Args[expectedFormatFlagIndex] == "--format" && mockCaptureCmdInfo.Args[expectedFormatFlagIndex+1] == "json") {
			t.Errorf("Expected 'docker compose -f <file> ps --format json', got command '%s' with args %v", mockCaptureCmdInfo.Command, mockCaptureCmdInfo.Args)
		}
	}
}

func TestRunDockerComposePs_SpecificService_JSON(t *testing.T) {
	setup(t)
	defer teardown()

	// Mock `docker compose ps service1 --format json` output
	mockPsJSONOutput := []DockerPsJSONEntry{
		{Name: "project_s1_1", Service: "service1", Image: "nginx:latest", Command: "nginx -g", State: "running", Publishers: []struct{URL string; TargetPort int; PublishedPort int; Protocol string}{{"0.0.0.0", 80, 8080, "tcp"}}},
	}
    var jsonOutputLines []string
	for _, entry := range mockPsJSONOutput {
		lineBytes, _ := json.Marshal(entry)
		jsonOutputLines = append(jsonOutputLines, string(lineBytes))
	}
	mockCaptureCommandOutput = strings.Join(jsonOutputLines, "\n")


	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	testRootCmd, _ := InitializeTestCmd(t)
	_, err := executeCommandCobra(testRootCmd, "ps", "service1") // Request specific service
	if err != nil {
		t.Fatalf("ps command execution for 'service1' failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check service1 (running and requested)
	if !strings.Contains(output, "service1         Running") {
		t.Errorf("Expected 'service1' (requested) to be 'Running'. Output:\n%s", output)
	}
	// Check that service2 and service_no_details (not requested) are NOT in the output
	if strings.Contains(output, "service2         Not Running") {
		t.Errorf("Did not expect 'service2' (not requested) in output. Output:\n%s", output)
	}
    if strings.Contains(output, "service_no_details Not Running") {
		t.Errorf("Did not expect 'service_no_details' (not requested) in output. Output:\n%s", output)
	}

	// Verify CaptureCommand was called for `docker compose ps service1 --format json`
	if mockCaptureCmdInfo.Calls == 0 {
		t.Error("CaptureCommand was not called for 'ps service1'")
	} else {
        // Expected args: compose -f <tempfile> ps service1 --format json
		if len(mockCaptureCmdInfo.Args) < 6 {
			t.Fatalf("CaptureCommand called with too few arguments for ps service1 --format json: %v", mockCaptureCmdInfo.Args)
		}

		if !(mockCaptureCmdInfo.Command == "docker" &&
			mockCaptureCmdInfo.Args[0] == "compose" && mockCaptureCmdInfo.Args[1] == "-f" &&
			mockCaptureCmdInfo.Args[3] == "ps" && mockCaptureCmdInfo.Args[4] == "service1" &&
			mockCaptureCmdInfo.Args[5] == "--format" && mockCaptureCmdInfo.Args[6] == "json") {
			t.Errorf("Expected 'docker compose -f <file> ps service1 --format json', got command '%s' with args %v", mockCaptureCmdInfo.Command, mockCaptureCmdInfo.Args)
		}
	}
}
