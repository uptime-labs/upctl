package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestCreateTempComposeFile_NoVersionKey(t *testing.T) {
	viper.Reset() // Ensure clean viper state

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
	// Load mock config into viper
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(strings.NewReader(mockYAMLConfig))
	if err != nil {
		t.Fatalf("Failed to read mock YAML config: %v", err)
	}

	// createTempComposeFile will call viper.Unmarshal into the global dockerComposeConfig.
	// The test should not do it beforehand.

	// Call the function to be tested
	tempFilePath, err := createTempComposeFile()
	if err != nil {
		t.Fatalf("createTempComposeFile() returned an error: %v", err)
	}
	defer os.Remove(tempFilePath) // Clean up the temporary file

	// Read the content of the generated temporary file
	generatedYAMLBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read temporary compose file '%s': %v", tempFilePath, err)
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
