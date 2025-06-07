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
            expectErrorLine: "Error: Failed to load or parse configuration. Attempted:", // It will also include the temp file path and Viper error details
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
            expectErrorLine: "Error: Failed to load or parse configuration. Attempted: default paths ($HOME/.upctl.yaml, ./.upctl.yaml). Viper error: Config File \".upctl\" Not Found",
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
            var explicitPathForTest string

            if tt.createMockFile {
                tempFile, err := os.CreateTemp(t.TempDir(), "test-*.yaml")
                if err != nil {
                    t.Fatalf("Failed to create temp file: %v", err)
                }
                if tt.configContent != "" { // Write content only if provided
                    _, err = tempFile.WriteString(tt.configContent)
                    if err != nil {
                        tempFile.Close() // Close before failing
                        t.Fatalf("Failed to write to temp file: %v", err)
                    }
                }
                tempFile.Close()
                explicitPathForTest = tempFile.Name()
            } else if tt.name == "Config file not found (simulated)" {
                explicitPathForTest = ""
            }

            output := captureOutput(func() {
                runValidationChecks(nil, []string{}, explicitPathForTest)
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
