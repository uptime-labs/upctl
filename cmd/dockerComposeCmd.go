package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// DockerComposeConfig is the struct that holds the Docker Compose config values
type DockerComposeConfig struct {
	Services map[string]interface{} `mapstructure:"services" yaml:"services"`
	Volumes  map[string]interface{} `mapstructure:"volumes" yaml:"volumes"`  // Removed omitempty
	Networks map[string]interface{} `mapstructure:"networks" yaml:"networks"` // Removed omitempty
}

var (
	dockerComposeConfig DockerComposeConfig
)

// RunDockerComposePs lists running docker compose services.
func RunDockerComposePs(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	fmt.Println("Listing Docker Compose services...")
	composeArgs := []string{"compose", "-f", tempComposePath, "ps"}
	composeArgs = append(composeArgs, args...) // Pass through any additional arguments

	err = ExecuteCommand("docker", composeArgs...)
	if err != nil {
		fmt.Printf("Error listing Docker Compose services: %s\n", err.Error())
		os.Exit(1)
	}
}

// RunDockerComposeUp starts docker compose services. It's public so it can be called from other packages.
func RunDockerComposeUp(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	// Create a temporary compose file
	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	// Start docker compose
	fmt.Println("Starting Docker Compose services...")

	var composeArgs []string
	composeArgs = append(composeArgs, "compose", "-f", tempComposePath, "up", "-d")

	// If a specific service is specified
	if len(args) > 0 {
		composeArgs = append(composeArgs, args[0])
	}

	err = ExecuteCommand("docker", composeArgs...)
	if err != nil {
		fmt.Printf("Error starting Docker Compose services: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Docker Compose services started successfully")
}

// RunDockerComposeDown stops Docker Compose services.
func RunDockerComposeDown(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	// Create a temporary compose file
	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	// Stop docker compose
	fmt.Println("Stopping Docker Compose services...")
	err = ExecuteCommand("docker", "compose", "-f", tempComposePath, "down")
	if err != nil {
		fmt.Printf("Error stopping Docker Compose services: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Docker Compose services stopped successfully")
}

// RunDockerComposeListServices lists available Docker Compose services defined in the configuration.
func RunDockerComposeListServices(cmd *cobra.Command, args []string) {
	// Load docker compose config from viper
	err := viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		fmt.Printf("Error loading docker compose config: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Available Docker Compose services:")
	for service := range dockerComposeConfig.Services {
		fmt.Printf("- %s\n", service)
	}
}

// RunDockerComposeInstall handles the installation of specific or all services.
func RunDockerComposeInstall(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	// Check if "all" flag is set or a specific service is requested
	installAll, _ := cmd.Flags().GetBool("all")

	if !(len(args) > 0 || installAll) {
		fmt.Println("Please provide a service name or --all flag")
		os.Exit(1)
	}

	// Create a temporary compose file
	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	// Load the services from the config
	err = viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		fmt.Printf("Error loading docker compose config: %s\n", err.Error())
		os.Exit(1)
	}

	if len(args) > 0 {
		// Check if the requested service exists
		serviceName := args[0]
		if _, exists := dockerComposeConfig.Services[serviceName]; !exists {
			fmt.Printf("Service '%s' not found in configuration\n", serviceName)
			os.Exit(1)
		}

		fmt.Printf("Installing and starting service: %s\n", serviceName)
		err = ExecuteCommand("docker", "compose", "-f", tempComposePath, "up", "-d", serviceName)
		if err != nil {
			fmt.Printf("Error installing service %s: %s\n", serviceName, err.Error())
			os.Exit(1)
		}
		fmt.Printf("Service %s installed and started successfully\n", serviceName)
	} else if installAll {
		fmt.Println("Installing and starting all services...")
		err = ExecuteCommand("docker", "compose", "-f", tempComposePath, "up", "-d")
		if err != nil {
			fmt.Printf("Error installing all services: %s\n", err.Error())
			os.Exit(1)
		}
		fmt.Println("All services installed and started successfully")
	}
}

// RunDockerComposeLogs shows logs for one or all services.
func RunDockerComposeLogs(cmd *cobra.Command, args []string) {
	// Create a temporary compose file
	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	var logArgs []string
	logArgs = append(logArgs, "compose", "-f", tempComposePath, "logs")

	// If a specific service is specified
	if len(args) > 0 {
		logArgs = append(logArgs, args[0])
	}

	// Show logs with follow option
	logArgs = append(logArgs, "--follow")

	// Execute the command to show logs
	err = ExecuteCommand("docker", logArgs...)
	if err != nil {
		fmt.Printf("Error showing logs: %s\n", err.Error())
		os.Exit(1)
	}
}

// createTempComposeFile creates a temporary docker-compose.yml file from the config
// RunDockerImportDB handles importing a database into a Docker MySQL container.
func RunDockerImportDB(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	// Create a temporary compose file
	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	// Make sure the MySQL service is running
	fmt.Println("Ensuring MySQL service is running...")
	err = ExecuteCommand("docker", "compose", "-f", tempComposePath, "up", "-d", "mysql")
	if err != nil {
		fmt.Printf("Error starting MySQL service: %s\n", err.Error())
		os.Exit(1)
	}

	// Handle database import using docker exec
	dbFilePath := cleanPath(mysqlConfig.DBFile)

	// If file does not exist, download from s3 bucket
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		// download database from s3 bucket using tsh aws command
		fmt.Println("Downloading database...")

		// Check for tsh and authenticate if needed
		path, err := exec.LookPath("tsh")
		if err != nil {
			fmt.Println("Error finding tsh:", err)
			progress.Stop()
			os.Exit(1)
		}

		// Authenticate with Teleport if needed
		fmt.Println("Authenticating with Teleport...")
		if err := ExecuteCommand(path, "login", fmt.Sprintf("--proxy=%s", teleportConfig.Host)); err != nil {
			fmt.Println("Error authenticating with Teleport:", err)
			progress.Stop()
			os.Exit(2)
		}

		fmt.Println("Authenticating with AWS...")
		if err := ExecuteCommand(path, "apps", "login", teleportConfig.AWSApp, "--aws-role", teleportConfig.AWSRole); err != nil {
			fmt.Println("Error authenticating with AWS:", err)
			progress.Stop()
			os.Exit(2)
		}

		// Download the database file
		if err := ExecuteCommand("tsh", "aws", "--app", teleportConfig.AWSApp, "s3", "cp",
			fmt.Sprintf("s3://%s/%s", mysqlConfig.S3Bucket, mysqlConfig.S3Key), dbFilePath,
			"--region", mysqlConfig.S3Region); err != nil {
			fmt.Println("Error downloading database:", err)
			progress.Stop()
			os.Exit(2)
		}
	}

	// Get the container ID
	fmt.Println("Getting MySQL container ID...")
	containerIDCmd := exec.Command("docker", "compose", "-f", tempComposePath, "ps", "-q", "mysql")
	containerIDBytes, err := containerIDCmd.Output()
	if err != nil {
		fmt.Printf("Error getting MySQL container ID: %s\n", err.Error())
		os.Exit(1)
	}

	containerID := strings.TrimSpace(string(containerIDBytes))
	if containerID == "" {
		fmt.Println("Error: MySQL container not found")
		os.Exit(1)
	}

	// Copy the database file to the container
	fmt.Println("Copying database file to container...")
	tmpPath := "/tmp/import.sql"
	err = ExecuteCommand("docker", "cp", dbFilePath, containerID+":"+tmpPath)
	if err != nil {
		fmt.Printf("Error copying database file to container: %s\n", err.Error())
		os.Exit(1)
	}

	// Import the database
	fmt.Println("Importing database...")
	importCmd := fmt.Sprintf("mysql -u %s -p%s %s < %s",
		mysqlConfig.User, mysqlConfig.Password, mysqlConfig.Database, tmpPath)

	err = ExecuteCommand("docker", "exec", containerID, "bash", "-c", importCmd)
	if err != nil {
		fmt.Printf("Error importing database: %s\n", err.Error())
		os.Exit(1)
	}

	// Clean up
	fmt.Println("Cleaning up...")
	err = ExecuteCommand("docker", "exec", containerID, "rm", tmpPath)
	if err != nil {
		fmt.Printf("Error cleaning up: %s\n", err.Error())
	}

	fmt.Println("Database imported successfully")
}

func createTempComposeFile() (string, error) {
	// Load docker compose config from viper
	err := viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		return "", fmt.Errorf("error loading docker compose config: %s", err.Error())
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "docker-compose-*.yml")
	if err != nil {
		return "", fmt.Errorf("error creating temporary file: %s", err.Error())
	}
	tempFilePath := tempFile.Name()

	// Marshal the config to YAML
	yamlData, err := yaml.Marshal(dockerComposeConfig)
	if err != nil {
		tempFile.Close()
		return "", fmt.Errorf("error marshaling config to YAML: %s", err.Error())
	}

	// Write the YAML to the temporary file
	if _, err := tempFile.Write(yamlData); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("error writing to temporary file: %s", err.Error())
	}

	// Close the file
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("error closing temporary file: %s", err.Error())
	}

	return tempFilePath, nil
}
