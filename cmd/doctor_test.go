package cmd

import (
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// Re-using captureOutput from validate_test.go (assuming it would be in a shared test_util.go or similar)
// For this subtask, I'll redefine it here.
func captureDoctorOutput(f func()) string {
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

func TestRunDoctorChecks(t *testing.T) {
	tests := []struct {
		name            string
		configContent   string
		setupExternal   func() (cleanup func()) // For setting up external conditions like occupied ports
		expectInOutput  []string
		notExpectInOutput []string
	}{
		{
			name: "Valid config, port available",
			configContent: `
services:
  web:
    image: myimage
    ports:
      - "8989:80"
`,
			expectInOutput: []string{"--- Upctl Doctor ---", "1. Checking config file... OK", "2. Validating config structure (services, volumes, networks)... OK", "3. Checking for 'services' definition... OK", "4. Checking for port conflicts...", "Info: Port 8989 (service: web, address: :8989) is available.", "--- Doctor checks complete ---"},
		},
		{
			name: "Config file not found",
			configContent: "", // Viper will be reset, no file set
			expectInOutput: []string{"Error: Config file not found."},
		},
		{
			name: "Invalid YAML syntax",
			configContent: "services: web: { image: myimage, ports: [\"8990:80\"] : } :", // Extra colons
			expectInOutput: []string{"Error: Could not read config file"},
		},
		{
			name: "Missing services key",
			configContent: "version: 1",
			expectInOutput: []string{"Error: 'services' key not found or empty"},
		},
		{
			name: "Internal port conflict",
			configContent: `
services:
  app1:
    image: app1_image
    ports:
      - "8991:80"
  app2:
    image: app2_image
    ports:
      - "8991:81"
`,
			expectInOutput: []string{"Info: Port 8991", "is available", "Error: Port 8991", "conflicts with service", "within upctl.yaml."},
		},
		{
			name: "Port already in use",
			configContent: `
services:
  testservice:
    image: testimg
    ports:
      - "8992:80"
`,
			setupExternal: func() (cleanup func()) {
				listener, err := net.Listen("tcp", ":8992")
				if err != nil {
					t.Logf("Could not listen on port 8992 for test setup: %v", err)
					return func() {} // No cleanup needed if listen failed
				}
				return func() { listener.Close() }
			},
			expectInOutput: []string{"Error: Port 8992 (service: testservice, address: :8992) is already in use on the host."},
		},
		{
            name: "Port defined with IP, available",
            configContent: `
services:
  web_with_ip:
    image: myimage
    ports:
      - "127.0.0.1:8993:80"
`,
            expectInOutput: []string{"Info: Port 8993 (service: web_with_ip, address: 127.0.0.1:8993) is available."},
        },
        {
            name: "Port defined with IP, in use",
            configContent: `
services:
  another_service:
    image: another_image
    ports:
      - "127.0.0.1:8994:80"
`,
            setupExternal: func() (cleanup func()) {
                listener, err := net.Listen("tcp", "127.0.0.1:8994")
                if err != nil {
                    t.Logf("Could not listen on port 127.0.0.1:8994 for test setup: %v", err)
                    return func() {}
                }
                return func() { listener.Close() }
            },
            expectInOutput: []string{"Error: Port 8994 (service: another_service, address: 127.0.0.1:8994) is already in use on the host."},
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()

			// Handle external setup (like occupying a port)
			if tt.setupExternal != nil {
				cleanup := tt.setupExternal()
				defer cleanup()
				// Give a moment for the port to be actually listened on
				time.Sleep(50 * time.Millisecond)
			}

			if tt.configContent != "" {
				tempFile, err := os.CreateTemp(t.TempDir(), "test-doctor-*.yaml")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				_, err = tempFile.WriteString(tt.configContent)
				if err != nil {
					tempFile.Close()
					t.Fatalf("Failed to write to temp file: %v", err)
				}
				tempFile.Close()
				viper.SetConfigFile(tempFile.Name())
				// ReadInConfig is called by runDoctorChecks or initConfig
			}
			// If configContent is "", viper is reset, so ConfigFileUsed() will be ""

			output := captureDoctorOutput(func() {
				// For doctor, initConfig is implicitly run by cobra if we were to execute the command.
				// Since we call runDoctorChecks directly, we simulate that initConfig would have read the config.
				// If viper.ConfigFileUsed() is empty after reset, runDoctorChecks should report it.
				// If SetConfigFile was called, runDoctorChecks will call ReadInConfig.
				if viper.ConfigFileUsed() != "" { // If a temp file was created and set
					// This call simulates what initConfig does before any command Run function.
					// It's important for runDoctorChecks's own ReadInConfig call.
					err := viper.ReadInConfig()
					if err != nil && tt.name != "Invalid YAML syntax" { // Allow "Invalid YAML syntax" to test the error handling in runDoctorChecks
						// t.Logf("Test setup: Viper failed to read mock config for %s: %v", tt.name, err)
						// This can happen if the config is intentionally malformed for a test case
					}
				}
				runDoctorChecks(nil, []string{})
			})

			for _, expected := range tt.expectInOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output for '%s' to contain '%s', but got:\n%s", tt.name, expected, output)
				}
			}
            if len(tt.notExpectInOutput) > 0 {
                for _, notExpected := range tt.notExpectInOutput {
                     if strings.Contains(output, notExpected) {
                        t.Errorf("Expected output for '%s' NOT to contain '%s', but got:\n%s", tt.name, notExpected, output)
                    }
                }
            }
		})
	}
}
