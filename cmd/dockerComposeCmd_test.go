package cmd

import (
	"bytes"
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
	"gopkg.in/yaml.v3"
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
			os.WriteFile(args[2], []byte("services: {}"), 0644)
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
			os.WriteFile(args[2], []byte("services: {}"), 0644)
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

	if _, err := tempCfgFile.WriteString(mockYAMLConfig); err != nil {
		tempCfgFile.Close()
		t.Fatalf("Failed to write to temp config file: %v", err)
	}
	if err := tempCfgFile.Close(); err != nil {
		t.Fatalf("Failed to close temp config file: %v", err)
	}

	cfgFile = tempCfgFile.Name() // Set for initConfig
	// initConfig will be called by InitializeTestCmd via cobra.OnInitialize
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
	viper.Reset()
	os.Exit(exitCode)
}

func executeCommandCobra(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute() // This will trigger OnInitialize (initConfig)
	return buf.String(), err
}


func TestRunDockerComposeUp_AllServices(t *testing.T) {
	setup(t)
	defer teardown()

	testRootCmd, _ := InitializeTestCmd()

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
	testRootCmd, _ := InitializeTestCmd()

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
func InitializeTestCmd() (*cobra.Command, error) {
    testRootCmd := &cobra.Command{Use: "upctl"}
    testRootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.upctl.yaml)")

    // This ensures that for this test command tree, initConfig will be called.
    // isTestInitCalled helps manage if cobra.OnInitialize needs to be (re)set globally.
    if !isTestInitCalled {
        cobra.OnInitialize(initConfig) // initConfig is from root.go
        isTestInitCalled = true
    }

    // Add commands needed for tests
    var testUpCmd = &cobra.Command{
        Use:   "up [service]",
        Short: "Start specified or all services",
        Args:  cobra.ArbitraryArgs,
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
	viper.Reset()

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
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(strings.NewReader(mockYAMLConfig))
	if err != nil {
		t.Fatalf("Failed to read mock YAML config for TestCreateTempComposeFile: %v", err)
	}
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
	err = yaml.Unmarshal(generatedYAMLBytes, &generatedContent)
	if err != nil {
		t.Fatalf("Failed to unmarshal generated YAML content: %v", err)
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
		} else if _, webExists := servicesMap["web"]; !webExists {
			t.Errorf("'services.web' not found in generated YAML")
		}
	}

	_, volumesExists := generatedContent["volumes"]
	if !volumesExists {
		t.Errorf("'volumes' key not found in generated docker-compose.yml")
	}

	_, networksExists := generatedContent["networks"]
	if !networksExists {
		t.Errorf("'networks' key not found in generated docker-compose.yml")
	}
}

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
	testRootCmd, _ := InitializeTestCmd()

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
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"logs", "--follow"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:5], expectedArgsSuffix) &&
			len(mockExecuteCmdInfo.Args) == 5) {
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
		expectedArgsPrefix := []string{"compose", "-f"}
		expectedArgsSuffix := []string{"logs", "--follow", "service1"}
		if !(mockExecuteCmdInfo.Command == "docker" &&
			equalSlices(mockExecuteCmdInfo.Args[:2], expectedArgsPrefix) &&
			equalSlices(mockExecuteCmdInfo.Args[3:6], expectedArgsSuffix) &&
			len(mockExecuteCmdInfo.Args) == 6) {
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

func TestRunDockerComposePs_CombinedOutput(t *testing.T) {
	setup(t)
	defer teardown()

	mockCaptureCmdInfo = MockExecuteCommandInfo{}
	mockCaptureCommandShouldError = false
	// service1 is in config and running, service2 in config and not running, service3 not in config but running
	mockCaptureCommandOutput = `NAME                IMAGE                             COMMAND                  SERVICE             CREATED             STATUS              PORTS
myproject-service1-1   nginx                             "nginx -g 'daemon of…"   service1            2 hours ago         Up 2 hours          0.0.0.0:80->80/tcp
myproject-service3-1   someotherimage                    "command"                service3            3 hours ago         Up 3 hours
`
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	testRootCmd, err := InitializeTestCmd()
	if err != nil {
		t.Fatalf("Failed to initialize test command: %v", err)
	}
	_, err = executeCommandCobra(testRootCmd, "ps") // This will trigger initConfig via OnInitialize
	if err != nil {
		t.Fatalf("ps command execution failed: %v", err)
	}


	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that viper was loaded correctly by initConfig
	if !viper.IsSet("services.service1") {
		t.Fatal("Viper config not loaded correctly for services.service1 in test")
	}
	if !viper.IsSet("services.service2") {
		t.Fatal("Viper config not loaded correctly for services.service2 in test")
	}


	if !strings.Contains(output, "--- Combined Service Status ---") {
		t.Errorf("Expected output to contain '--- Combined Service Status ---', got:\n%s", output)
	}
	if !strings.Contains(output, "service1         Running") {
		t.Errorf("Expected output to show service1 as Running, got:\n%s", output)
	}
	if !strings.Contains(output, "myproject-service1-1") {
		t.Errorf("Expected output to contain ps details for service1 (e.g., NAME), got:\n%s", output)
	}
	if !strings.Contains(output, "service2         Not Running") {
		t.Errorf("Expected output to show service2 as Not Running, got:\n%s", output)
	}

	lines := strings.Split(output, "\n")
	configServiceColumnHeader := "CONFIG SERVICE"
	headerIndex := -1
	for i, line := range lines {
		if strings.HasPrefix(line, configServiceColumnHeader) {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		if strings.Contains(output, "Error creating temporary compose file") {
			t.Fatalf("TestRunDockerComposePs failed because createTempComposeFile errored. Output:\n%s", output)
		}
		t.Fatalf("Could not find the table header '%s' in output:\n%s", configServiceColumnHeader, output)
	}

	for i := headerIndex + 1; i < len(lines); i++ {
		trimmedLine := strings.TrimSpace(lines[i])
		if trimmedLine == "" { continue }
		if strings.HasPrefix(trimmedLine, "service3") {
			t.Errorf("Service 'service3' (not in config) should not be listed as a primary config service. Found line: %s", lines[i])
			break
		}
	}

	if mockCaptureCmdInfo.Calls == 0 {
		t.Error("CaptureCommand was not called for 'ps'")
	} else {
		if len(mockCaptureCmdInfo.Args) < 4 {
			t.Fatalf("CaptureCommand called with too few arguments: %v", mockCaptureCmdInfo.Args)
		}
		expectedArgsPrefix := []string{"compose", "-f"}
		if !(mockCaptureCmdInfo.Command == "docker" &&
			equalSlices(mockCaptureCmdInfo.Args[:2], expectedArgsPrefix) &&
			mockCaptureCmdInfo.Args[3] == "ps" &&
			len(mockCaptureCmdInfo.Args) == 4) {
			t.Errorf("Expected 'docker compose -f <file> ps', got command '%s' with args %v", mockCaptureCmdInfo.Command, mockCaptureCmdInfo.Args)
		}
	}
}
