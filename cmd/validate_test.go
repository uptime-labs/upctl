package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Helper function to capture stdout
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestRunValidationChecks(t *testing.T) {
	// Store original os.Args and defer restoration
	// Note: cobra execution might not directly use os.Args for Run functions in tests,
	// but good practice if any underlying code might.
	// For these tests, we call runValidationChecks directly.

	// Backup and defer restore of viper's state if necessary, or use viper.Reset()
	// For simplicity, we'll re-initialize viper for each sub-test as needed.

	tests := []struct {
		name           string
		configContent  string
		setupViper     func(content string) // Function to set up viper for the test
		expectError    bool
		expectedOutput []string // Multiple strings to check for in output
	}{
		{
			name: "Valid upctl.yaml",
			configContent: `
services:
  myservice:
    image: myimage
mysql:
  host: localhost
teleport:
  host: localhost
docker_config:
  registry: myregistry
`,
			setupViper: func(content string) {
				viper.Reset()
				viper.SetConfigType("yaml")
				viper.ReadConfig(strings.NewReader(content))
				// Simulate ConfigFileUsed being set, as initConfig would do this
				viper.Set("internal_test_config_file_used", "dummy_valid.yaml")
			},
			expectError: false,
			expectedOutput: []string{"Validating upctl.yaml...", "Using configuration file:", "YAML syntax: OK", "Overall structure: OK", "'services' key: Present", "upctl.yaml is valid."},
		},
		{
			name:          "Invalid YAML syntax",
			configContent: "services: \n  myservice: \n    image: myimage\n  another: broken_syntax:",
			setupViper: func(content string) {
				viper.Reset()
				viper.SetConfigType("yaml")
				// ReadConfig will error, but that's what we are testing runValidationChecks's handling of.
				// Forcing an error for ReadInConfig within runValidationChecks.
				// We can't easily make viper.ReadInConfig inside runValidationChecks fail if the initial read here succeeds.
				// So, this test relies on runValidationChecks itself calling ReadInConfig.
				// The best way is to ensure viper thinks a file is set, but its content is bad.
				f, _ := os.CreateTemp("", "bad_yaml_*.yaml")
				f.WriteString(content)
				f.Close()
				viper.SetConfigFile(f.Name())
				// defer os.Remove(f.Name()) // Clean up, but might be removed before ReadInConfig if test ends quickly
			},
			expectError:    true, // The function itself doesn't return error, but prints it
			expectedOutput: []string{"Validating upctl.yaml...", "Error: Failed to read or parse configuration file."},
		},
		{
			name: "Missing services key",
			configContent: `
volumes:
  myvolume: {}
`,
			setupViper: func(content string) {
				viper.Reset()
				viper.SetConfigType("yaml")
				viper.ReadConfig(strings.NewReader(content))
				viper.Set("internal_test_config_file_used", "dummy_missing_services.yaml")
			},
			expectError:    true,
			expectedOutput: []string{"Validating upctl.yaml...", "Error: The 'services' key is missing or empty"},
		},
		{
			name:          "Config file not found",
			configContent: "", // No content, viper will be set to a non-existent file
			setupViper: func(content string) {
				viper.Reset()
				viper.SetConfigFile("non_existent_config.yaml")
				// We want viper.ConfigFileUsed() to return our non-existent path
				// but viper.ReadInConfig() within runValidationChecks to fail as "file not found".
				// This setup is tricky because initConfig usually handles the initial load and exit.
				// Forcing ConfigFileUsed() to be empty is a more direct test of one branch in runValidationChecks.
				// This will be simulated by viper.Get("internal_test_config_file_used") returning ""
			},
			expectError:    true,
			expectedOutput: []string{"Validating upctl.yaml...", "Error: Configuration file not found."},
		},
		{
			name: "Invalid structure (e.g. services is not a map)",
			configContent: `
services: "not a map"
`,
			setupViper: func(content string) {
				viper.Reset()
				viper.SetConfigType("yaml")
				viper.ReadConfig(strings.NewReader(content))
				viper.Set("internal_test_config_file_used", "dummy_invalid_structure.yaml")
			},
			expectError: true,
			expectedOutput: []string{"Validating upctl.yaml...", "Error: Configuration file structure is invalid."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Original viper.ConfigFileUsed() can't be easily mocked without changing root.go
			// So we use a workaround for tests needing specific ConfigFileUsed() behavior.
			// The "Config file not found" case specifically.
			originalConfigFileUsed := viper.ConfigFileUsed()

			if tt.name == "Config file not found" {
				viper.Reset() // Ensure no config file is actually loaded or set
				// To simulate viper.ConfigFileUsed() == "" inside runValidationChecks,
				// we are relying on a fresh viper instance where no config file has been successfully read or set.
				// The check `if viper.ConfigFileUsed() == ""` in `runValidationChecks` will be true.
			} else if tt.name == "Invalid YAML syntax" {
                 // Special handling for bad YAML syntax test
                tt.setupViper(tt.configContent)
                // defer os.Remove(viper.ConfigFile()) // Ensure cleanup after test
            } else {
				tt.setupViper(tt.configContent)
			}


			output := captureOutput(func() {
				// Directly call runValidationChecks.
				// For a more integrated test, you'd use rootCmd.SetArgs(...) and rootCmd.Execute(),
				// but that requires more setup for arg parsing and command matching.
				runValidationChecks(nil, []string{}) // cmd and args are not used by runValidationChecks
			})

			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', but got:\n%s", expected, output)
				}
			}

			// Restore original viper config file if changed, though Reset should handle most cases
			if tt.name == "Config file not found" || tt.name == "Invalid YAML syntax" {
				viper.Reset() // Clean up viper state
				if originalConfigFileUsed != "" { // If there was a global config, try to restore it
					viper.SetConfigFile(originalConfigFileUsed)
					viper.ReadInConfig()
				}
			}
		})
	}
}

// Mocking viper.ConfigFileUsed() is hard. The test for "Config file not found"
// will rely on viper being reset and no config file being set, so viper.ConfigFileUsed()
// inside the function (if it uses the global viper) returns empty.
// The `internal_test_config_file_used` was an attempt to control this, but `viper.ConfigFileUsed()`
// is a direct viper function. Instead, for tests needing specific ConfigFileUsed() behavior,
// we ensure viper's internal state reflects that.

// For "Invalid YAML syntax", the setup creates a temporary file with bad YAML.
// `viper.SetConfigFile` points to it. `runValidationChecks` calls `viper.ReadInConfig()`, which fails.

// For other tests, `viper.ReadConfig(strings.NewReader(content))` loads the config.
// `viper.ConfigFileUsed()` would be empty unless we set a dummy file path.
// The `runValidationChecks` uses `viper.ConfigFileUsed()` for its first check.
// We can modify `runValidationChecks` to accept `configFileUsed string` as a parameter for easier testing,
// or accept that testing that specific print line is harder with direct function calls.
// The current workaround for "Valid" and "Missing services key" is to set a dummy value
// via `viper.Set("internal_test_config_file_used", "dummy.yaml")` and modify `runValidationChecks`
// to use `viper.GetString("internal_test_config_file_used")` IF it's for testing.
// This is not ideal.
// A better approach for testing ConfigFileUsed:
// In tests that need to simulate a config file being used (like "Valid"):
// Create a temporary valid YAML file, use viper.SetConfigFile(tempFileName), then viper.ReadInConfig().
// Then viper.ConfigFileUsed() will return tempFileName.
// This is what the "Invalid YAML syntax" test does. Let's adapt other tests.

func TestRunValidationChecks_Refined(t *testing.T) {
    tests := []struct {
        name            string
        configContent   string
        createMockFile  bool // True if the content should be written to a temp file
        expectErrorLine string // A specific line indicating an error, if expected
        notExpectedErrorLine string // A specific line that should NOT be present if test passes
        expectedLine    string // A specific line indicating success, if applicable
    }{
        {
            name: "Valid upctl.yaml",
            configContent: `
services:
  myservice: {image: myimage}
mysql: {host: localhost}
teleport: {host: localhost}
docker_config: {registry: myregistry}
`,
            createMockFile: true,
            expectedLine:   "upctl.yaml is valid.",
        },
        {
            name: "Invalid YAML syntax",
            configContent:  "services: \n  myservice: \n    image: myimage\n  another: broken_syntax:",
            createMockFile: true,
            expectErrorLine: "Error: Failed to read or parse configuration file.",
        },
        {
            name: "Missing services key",
            configContent: `
volumes: {myvolume: {}}
mysql: {host: localhost}
teleport: {host: localhost}
docker_config: {registry: myregistry}
`,
            createMockFile: true,
            expectErrorLine: "Error: The 'services' key is missing or empty",
        },
        {
            name: "Config file not found (simulated)",
            configContent:  "", // No content needed
            createMockFile: false, // No file will be set in viper
            expectErrorLine: "Error: Configuration file not found.",
        },
        {
            name: "Invalid structure (services is not a map)",
            configContent: `
services: "not a map"
mysql: {host: localhost}
teleport: {host: localhost}
docker_config: {registry: myregistry}
`,
            createMockFile: true,
            expectErrorLine: "Error: Configuration file structure is invalid.",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            viper.Reset()

            if tt.createMockFile {
                tempFile, err := os.CreateTemp(t.TempDir(), "test-*.yaml")
                if err != nil {
                    t.Fatalf("Failed to create temp file: %v", err)
                }
                if tt.configContent != "" {
                    _, err = tempFile.WriteString(tt.configContent)
                    if err != nil {
                        t.Fatalf("Failed to write to temp file: %v", err)
                    }
                }
                tempFile.Close() // Close the file so viper can read it
                viper.SetConfigFile(tempFile.Name())
                // Note: ReadInConfig is called by runValidationChecks
            } else if tt.name == "Config file not found (simulated)" {
                // For this case, viper is reset, and no config file is set.
                // initConfig in root.go would normally run and set a default or exit.
                // Here, we are testing runValidationChecks in a somewhat isolated manner.
                // The check `configFileUsed := viper.ConfigFileUsed()` inside `runValidationChecks`
                // should return empty.
            }


            output := captureOutput(func() {
                runValidationChecks(nil, []string{})
            })

            if tt.expectErrorLine != "" {
                if !strings.Contains(output, tt.expectErrorLine) {
                    t.Errorf("Expected output to contain error '%s', but got:\n%s", tt.expectErrorLine, output)
                }
            }
            if tt.notExpectedErrorLine != "" {
                 if strings.Contains(output, tt.notExpectedErrorLine) {
                    t.Errorf("Expected output NOT to contain '%s', but got:\n%s", tt.notExpectedErrorLine, output)
                }
            }
            if tt.expectedLine != "" {
                if !strings.Contains(output, tt.expectedLine) {
                    t.Errorf("Expected output to contain '%s', but got:\n%s", tt.expectedLine, output)
                }
            }
        })
    }
}

// Note: The actual execution of these tests depends on the behavior of initConfig in cmd/root.go.
// If initConfig exits on error, some of these test cases might not run as expected when testing
// via `go test ./cmd`. The `runValidationChecks` is called directly here, bypassing some of
// cobra's and initConfig's upfront error handling for a more unit-style test of the function's logic.
// The "Config file not found (simulated)" case specifically relies on `viper.ConfigFileUsed()` being empty
// when `runValidationChecks` starts, which might conflict with `initConfig` if it always ensures a
// (default) config path is set in viper or exits.
// For the purpose of this subtask, this direct call and viper manipulation is a common way to unit test such functions.
// A real test suite might involve building a main_test.go to run `TestMain` and control cobra/viper setup more globally.

// Further refinement: For "Config file not found", viper.ConfigFileUsed() will only be empty
// if viper.ReadInConfig() has not successfully found and read a file OR if SetConfigFile was never called.
// initConfig in root.go calls viper.ReadInConfig and os.Exit(1) if it's not found/readable.
// To test the "Error: Configuration file not found." path in runValidationChecks, we'd need to ensure
// that initConfig somehow doesn't run or exit before runValidationChecks. This is tricky.
// The most reliable way this branch in runValidationChecks is hit is if --config some_non_existent_file.yaml is passed.

// Let's assume for these tests that `initConfig` has not pre-emptively exited for "Config file not found".
// The `viper.Reset()` at the start of each test case is key.

// The `TestRunValidationChecks_Refined` is a better structure. I'll use that.
// I need to remove the older `TestRunValidationChecks`.
// The placeholder `services: {image: myimage}` should be `services: myservice: {image: myimage}` to be valid.
// Corrected in the refined test case.
// The `mysql: {host: localhost}` etc. also need a key like `mysql: mymysql: {host:localhost}` if they are maps of services,
// but they are defined as direct structs in UpctlConfigForValidation, so `key: {fields}` is correct.
// Example: `mysql: {host: localhost, database: db1}`
// The refined test cases look more robust.

// Final structure of the file:
// package cmd
// imports
// captureOutput func
// TestRunValidationChecks_Refined func (containing the test table and logic)

// This will be the content of cmd/validate_test.go
// I will remove the old TestRunValidationChecks from the block.I've created the initial structure for `cmd/validate_test.go` including a helper for capturing output and a table-driven test `TestRunValidationChecks_Refined`. This refined test structure is better as it uses temporary files for most cases, which more closely mimics how Viper would load configurations.

The key challenge in testing `runValidationChecks` in complete isolation is its reliance on the global Viper state, which is typically initialized by `initConfig` in `cmd/root.go`. `initConfig` itself has error handling that might `os.Exit(1)`, potentially preventing `runValidationChecks` from being called in certain scenarios if tested via full `rootCmd.Execute()` in a test. By calling `runValidationChecks` directly and manipulating Viper's state within each test case (especially using `viper.Reset()` and `viper.SetConfigFile()`), we can achieve more targeted unit testing of its logic.

The "Config file not found (simulated)" case is the most sensitive to this. If `initConfig` always ensures some config file is found or exits, then the branch `if configFileUsed == ""` inside `runValidationChecks` might seem hard to reach. However, `viper.Reset()` ensures `viper.ConfigFileUsed()` will be empty if no subsequent `viper.SetConfigFile()` and successful `viper.ReadInConfig()` happens *within the scope of the test function itself or the function being tested*. Since `runValidationChecks` itself calls `viper.ReadInConfig()`, the critical part for this specific test case is that `viper.ConfigFileUsed()` is checked *before* the `ReadInConfig` call within `runValidationChecks`.

Let's ensure the `cmd/validate_test.go` is created with the refined test structure.
