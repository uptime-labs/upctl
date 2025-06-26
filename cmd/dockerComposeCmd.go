package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// DockerComposeConfig is the struct that holds the Docker Compose config values
type DockerComposeConfig struct {
	Services map[string]interface{} `mapstructure:"services" yaml:"services"`
	Volumes  map[string]interface{} `mapstructure:"volumes" yaml:"volumes"`
	Networks map[string]interface{} `mapstructure:"networks" yaml:"networks"`
}

var (
	dockerComposeConfig DockerComposeConfig
)

// RunDockerComposePs lists running docker compose services and available services from config.
func RunDockerComposePs(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	// Get available services from config
	err := viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		fmt.Printf("Error loading docker compose config: %s\n", err.Error())
		// Decide if we should exit or try to continue with just ps output
	}
	availableServices := make(map[string]bool)
	for serviceName := range dockerComposeConfig.Services {
		availableServices[serviceName] = true
	}

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1) // Essential for ps command
	}
	defer os.Remove(tempComposePath)

	fmt.Println("Listing Docker Compose services (running and available)...")
	composePsArgs := []string{"compose", "-f", tempComposePath, "ps"}
	// Pass through any additional arguments like --all, --format, etc.
	// However, we will be processing the output, so some flags might conflict.
	// For now, let's stick to the default `ps` output and allow filtering by service names from args.
	// If args are provided, they are typically service names for `docker compose ps [SERVICE...]`
	if len(args) > 0 {
		composePsArgs = append(composePsArgs, args...)
	}

	// Capture the output of docker compose ps
	psOutput, err := CaptureCommand("docker", composePsArgs...)
	if err != nil {
		// If `docker compose ps` fails (e.g., no containers running), it might still be useful to show available services.
		fmt.Printf("Note: Error executing 'docker compose ps': %s. Output might be incomplete.\n", err.Error())
		// psOutput will be empty or contain error messages, which is fine for parsing.
	}

	// Parse the psOutput
	// The output typically looks like:
	// NAME                IMAGE                             COMMAND                  SERVICE             CREATED             STATUS              PORTS
	// t-mqtt-1            eclipse-mosquitto:latest          "/docker-entrypoint.…"   mqtt                7 days ago          Up 5 hours          0.0.0.0:1883->1883/tcp, :::1883->1883/tcp, 0.0.0.0:9001->9001/tcp, :::9001->9001/tcp
	// ...
	// We need to identify running services and their details.
	// A simple approach is to check which of the `availableServices` appear in the `psOutput`.

	lines := strings.Split(strings.TrimSpace(psOutput), "\n")
	runningServicesDetails := make(map[string]string) // map service name to its ps output line

	if len(lines) > 1 { // Header + data lines
		header := lines[0]
		// Try to find the "SERVICE" column to robustly get the service name
		// This is a basic heuristic. A more robust parser might be needed for complex `ps` outputs.
		serviceColIndex := -1
		// A common ps format has SERVICE as one of the columns. We can try to find it.
		// For now, we'll assume the service name from docker-compose config is what `docker compose ps` uses for the SERVICE column.
		// Or, sometimes the NAME column is <project>_<service>_1.
		// Let's iterate through available services and check if they are in the output lines.

		for serviceNameFromConfig := range availableServices {
			for i := 1; i < len(lines); i++ {
				// A common pattern is that the "SERVICE" column in `docker compose ps` matches the service name in `docker-compose.yml`
				// Or the "NAME" column is like `projectname-servicename-1`
				// We will check if the line contains the service name as a distinct word or part of the NAME.
				// This is a heuristic and might need refinement.
				// Example ps line: myproject-redis-1   redis:latest   "docker-entrypoint.s…"   redis    ...
				// Here, "redis" is the service name.
				// We can look for ` ` + serviceNameFromConfig + ` ` or ` ` + serviceNameFromConfig + `\n` (end of line)
				// Or check if NAME column starts with `projectprefix` + `serviceNameFromConfig`
				// For simplicity, let's assume SERVICE column is reliable or NAME column contains the service name.
				// A robust way is to parse each line by columns.
				// For now, we'll use a simpler string search on the line for the service name.
				// This might lead to false positives if a service name is a substring of another's details.

				// More robust: split line by multiple spaces, look for SERVICE column.
				// For now, let's assume the default ps output format is relatively stable.
				// We'll try to match the service name from config against the "SERVICE" column if present,
				// or against the "NAME" column (expecting it to be like project_service_replica).

				// Let's try to parse based on the "SERVICE" column first.
				// We need to find the column index for "SERVICE"
				if serviceColIndex == -1 && i == 1 { // Only determine column index from header once
					// A more flexible way to find column index
					re := regexp.MustCompile(`\s\s+`) // split by 2 or more spaces
					headerParts := re.Split(strings.TrimSpace(header), -1)
					for idx, part := range headerParts {
						if strings.ToUpper(part) == "SERVICE" {
							serviceColIndex = idx
							break
						}
					}
				}

				lineParts := regexp.MustCompile(`\s\s+`).Split(strings.TrimSpace(lines[i]), -1)
				psServiceName := ""
				if serviceColIndex != -1 && len(lineParts) > serviceColIndex {
					psServiceName = lineParts[serviceColIndex]
				} else if len(lineParts) > 0 {
					// Fallback: check if the NAME column (first column typically) contains the service name
					// e.g. NAME is "project_myservice_1", serviceNameFromConfig is "myservice"
					// This is less precise. The SERVICE column is better.
					// For now, let's rely on the SERVICE column or direct string match if SERVICE column isn't found.
					// If SERVICE column is not found, we rely on the original simpler check.
					if strings.Contains(lines[i], " "+serviceNameFromConfig+" ") || strings.HasSuffix(lines[i], " "+serviceNameFromConfig) {
						psServiceName = serviceNameFromConfig // Assume match
					}
				}


				if psServiceName == serviceNameFromConfig {
					runningServicesDetails[serviceNameFromConfig] = lines[i]
					break // Found this service, move to next from config
				}
			}
		}
	}


	// Print combined output
	// Header: SERVICE_CONFIG_NAME, STATUS, DOCKER_PS_OUTPUT_IF_RUNNING (or individual fields)
	// For now, let's print the full ps line for running services, and "Not Running" for others.
	// A more structured table would be better.
	// NAME, IMAGE, COMMAND, SERVICE, CREATED, STATUS, PORTS (from original ps)
	// We add: CONFIG_SERVICE_NAME, ACTUAL_STATUS (Running/Not Running), ... then details from ps if running

	fmt.Println("\n--- Combined Service Status ---")
	// Attempt to print a structured table
	// Header based on typical `docker compose ps`
	// We need to handle cases where psOutput is empty or only has a header.
	headerToPrint := "CONFIG SERVICE   STATUS         NAME             IMAGE                      COMMAND                  SERVICE (PS)      CREATED             PS STATUS           PORTS"
	if len(lines) > 0 && strings.Contains(lines[0], "NAME") { // Use actual header if available
		// We need to prepend "CONFIG SERVICE   STATUS      "
		// headerToPrint = "CONFIG SERVICE   STATUS         " + lines[0]
		// This might be too wide. Let's define our own for now.
	}
	fmt.Println(headerToPrint)
	fmt.Println(strings.Repeat("-", len(headerToPrint)))


	// Sort available service names for consistent output order
	sortedServiceNames := make([]string, 0, len(availableServices))
	for name := range availableServices {
		sortedServiceNames = append(sortedServiceNames, name)
	}
	sort.Strings(sortedServiceNames)

	for _, serviceName := range sortedServiceNames {
		if details, isRunning := runningServicesDetails[serviceName]; isRunning {
			// Parse the details string to align with our header
			// NAME, IMAGE, COMMAND, SERVICE, CREATED, STATUS, PORTS
			// Example line: t-mqtt-1         eclipse-mosquitto:latest   "/docker-entrypoint.…"   mqtt         6 days ago           Up 2 hours          0.0.0.0:1883->1883/tcp...
			// This parsing needs to be robust. Using `text/tabwriter` might be good here.
			// For now, a simple split by multiple spaces.
			parts := regexp.MustCompile(`\s\s+`).Split(strings.TrimSpace(details), -1)
			// Ensure parts has enough elements before accessing them
			name := ""
			image := ""
			command := ""
			psService := ""
			created := ""
			psStatus := ""
			ports := ""

			if len(parts) > 0 { name = parts[0] }
			if len(parts) > 1 { image = parts[1] }
			if len(parts) > 2 { command = parts[2] }
			if len(parts) > 3 { psService = parts[3] }
			if len(parts) > 4 { created = parts[4] }
			if len(parts) > 5 { psStatus = parts[5] }
			if len(parts) > 6 { ports = strings.Join(parts[6:], " ") } // Ports can contain spaces

			// Truncate command for display
			if len(command) > 20 { command = command[:17] + "..." }


			// Using fmt.Printf with fixed widths for basic alignment
			// Adjust widths as needed
			fmt.Printf("%-16s %-14s %-16s %-26s %-24s %-17s %-19s %-19s %s\n",
				serviceName,
				"Running",
				name,
				image,
				command,
				psService,
				created,
				psStatus,
				ports,
			)
		} else {
			// Service is in config but not in `docker compose ps` output
			fmt.Printf("%-16s %-14s %s\n", serviceName, "Not Running", strings.Repeat(" ", 110)) // Pad to align somewhat
		}
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

	allServices, _ := cmd.Flags().GetBool("all") // Get the --all flag

	var composeArgs []string
	composeArgs = append(composeArgs, "compose", "-f", tempComposePath, "up", "-d")

	// If a specific service is specified (and --all is not set)
	// The validation in cmd/root.go's upCmd ensures that if allServices is true, len(args) will be 0.
	// And if allServices is false, len(args) will be 1.
	if !allServices && len(args) > 0 {
		composeArgs = append(composeArgs, args[0])
	}
	// If allServices is true, args will be empty, and no service name is appended, so all services defined in the compose file are started.

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

	allServices, _ := cmd.Flags().GetBool("all")

	composeArgs := []string{"compose", "-f", tempComposePath, "down"}

	// If a specific service is specified (and --all is not set)
	// Validation in root.go ensures that if !allServices, len(args) == 1.
	// If allServices is true, len(args) == 0.
	if !allServices && len(args) > 0 {
		// As per subtask, add service name. Note: `docker compose down <service>` is not standard.
		// Standard commands are `stop <service>` then `rm <service>`.
		// This will likely result in `down` ignoring the service name or erroring.
		// However, fulfilling the request.
		composeArgs = append(composeArgs, args[0])
	}
	// If allServices is true, no service name is appended.

	err = ExecuteCommand("docker", composeArgs...)
	if err != nil {
		fmt.Printf("Error stopping Docker Compose services: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Docker Compose services stopped successfully")
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

	allServicesLogs, _ := cmd.Flags().GetBool("all")

	var logArgs []string
	logArgs = append(logArgs, "compose", "-f", tempComposePath, "logs", "--follow")

	// If a specific service is specified (and --all is not set)
	// Validation in root.go ensures that if !allServicesLogs, len(args) == 1.
	// If allServicesLogs is true, len(args) == 0.
	if !allServicesLogs && len(args) > 0 {
		logArgs = append(logArgs, args[0])
	}
	// If allServicesLogs is true, no service name is appended, showing all logs.

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
		// Even if general unmarshal fails, proceed to try to get specific keys.
		// Or, return error: return "", fmt.Errorf("error initial unmarshal: %s", err.Error())
		// For this fix, let's assume specific Get calls are the source of truth for these keys.
		// Initialize struct fields if they are nil (they should be if Unmarshal failed or didn't populate)
		if dockerComposeConfig.Services == nil {
			dockerComposeConfig.Services = make(map[string]interface{})
		}
		if dockerComposeConfig.Volumes == nil {
			dockerComposeConfig.Volumes = make(map[string]interface{})
		}
		if dockerComposeConfig.Networks == nil {
			dockerComposeConfig.Networks = make(map[string]interface{})
		}
	}

	// Explicitly ensure top-level keys are populated
	if viper.IsSet("services") {
		servicesData := viper.Get("services")
		if servicesMap, ok := servicesData.(map[string]interface{}); ok {
			dockerComposeConfig.Services = servicesMap
		} else {
			fmt.Fprintf(os.Stderr, "Warning: 'services' key in upctl.yaml is not in the expected map[string]interface{} format.\n")
		}
	}

	if viper.IsSet("volumes") {
		volumesData := viper.Get("volumes")
		if volumesMap, ok := volumesData.(map[string]interface{}); ok {
			dockerComposeConfig.Volumes = volumesMap
		} else {
			fmt.Fprintf(os.Stderr, "Warning: 'volumes' key in upctl.yaml is not in the expected map[string]interface{} format.\n")
		}
	}

	if viper.IsSet("networks") {
		networksData := viper.Get("networks")
		if networksMap, ok := networksData.(map[string]interface{}); ok {
			dockerComposeConfig.Networks = networksMap
		} else {
			fmt.Fprintf(os.Stderr, "Warning: 'networks' key in upctl.yaml is not in the expected map[string]interface{} format.\n")
		}
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
