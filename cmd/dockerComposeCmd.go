package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	// "regexp" // Not strictly needed anymore with JSON parsing for main fields
	"sort"
	"strings"
	// "text/tabwriter" // Consider for more advanced table formatting if needed

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

// DockerPsJSONEntry defines the structure for a single service entry from `docker compose ps --format json`
type DockerPsJSONEntry struct {
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	Image      string `json:"Image"`
	Command    string `json:"Command"`
	Project    string `json:"Project"`
	Service    string `json:"Service"` // This is the crucial key linking to config service name
	State      string `json:"State"`   // e.g., "running", "exited"
	Status     string `json:"Status"`  // e.g., "Up 2 hours", "Exited (0) 2 minutes ago"
	Health     string `json:"Health"`
	ExitCode   int    `json:"ExitCode"`
	Publishers []struct {
		URL           string `json:"URL"`
		TargetPort    int    `json:"TargetPort"`
		PublishedPort int    `json:"PublishedPort"`
		Protocol      string `json:"Protocol"`
	} `json:"Publishers"`
}

var (
	dockerComposeConfig DockerComposeConfig
)

// RunDockerComposePs lists running docker compose services and available services from config using JSON output.
func RunDockerComposePs(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	// Get available services from config
	err := viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		fmt.Printf("Error loading docker compose config: %s\n", err.Error())
	}
	availableServicesFromConfig := make(map[string]bool)
	for serviceName := range dockerComposeConfig.Services {
		availableServicesFromConfig[serviceName] = true
	}

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	fmt.Println("Listing Docker Compose services (running and available)...")
	composePsBaseArgs := []string{"compose", "-f", tempComposePath}
	composePsCmdArgs := append(composePsBaseArgs, "ps")
	if len(args) > 0 {
		composePsCmdArgs = append(composePsCmdArgs, args...)
	}
	composePsCmdArgs = append(composePsCmdArgs, "--format", "json")

	psOutputStr, err := CaptureCommand("docker", composePsCmdArgs...)
	if err != nil {
		fmt.Printf("Note: Error executing 'docker compose ps --format json': %s. Output might be incomplete.\n", err.Error())
	}

	runningServicesDetails := make(map[string]DockerPsJSONEntry)
	scanner := bufio.NewScanner(strings.NewReader(psOutputStr))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry DockerPsJSONEntry
		if errUnmarshal := json.Unmarshal([]byte(line), &entry); errUnmarshal != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse JSON line from 'docker compose ps': %v\nLine: %s\n", errUnmarshal, line)
			continue
		}
		if entry.Service != "" {
			runningServicesDetails[entry.Service] = entry
		}
	}
	if errScan := scanner.Err(); errScan != nil {
		fmt.Fprintf(os.Stderr, "Error reading 'docker compose ps' output stream: %v\n", errScan)
	}

	fmt.Println("\n--- Combined Service Status ---")
	headerToPrint := "CONFIG SERVICE   STATUS         NAME             IMAGE                      COMMAND                  SERVICE (PS)      STATE               PORTS"
	fmt.Println(headerToPrint)
	fmt.Println(strings.Repeat("-", len(headerToPrint)))

	servicesToDisplay := make([]string, 0)
	if len(args) > 0 {
		tempServicesToDisplay := make(map[string]bool)
		for _, argService := range args {
			if availableServicesFromConfig[argService] {
				tempServicesToDisplay[argService] = true
			} else {
				// If user specifies a service not in upctl.yaml, `docker compose ps <service_not_in_config>`
				// would have already filtered it or errored. If it somehow appears in JSON (e.g. part of project but not config),
				// we might choose to display it or ignore it. For now, we only focus on config-known services.
				// However, the `docker compose ps <service_name>` call might return valid JSON for a service not in upctl.yaml
				// if it's part of the same docker-compose project.
				// The current loop `for _, serviceName := range servicesToDisplay` will only iterate over services
				// that are either in the config (if len(args)==0) or specified in args AND in config.
				// Let's adjust to allow displaying a service specified in args even if not in config, if docker returns it.
				// This means `servicesToDisplay` should just be `args` if `len(args) > 0`.
				// The "CONFIG SERVICE" column might be misleading then.
				// For now, sticking to: if args provided, only show those *if they are also in config*.
				// This makes "CONFIG SERVICE" column meaningful.
				// If a user *really* wants to see a non-config service, they can use `docker compose ps` directly.
				// This upctl command is about managing *configured* services.
				fmt.Printf("Info: Service '%s' specified in 'ps' command is not defined in upctl.yaml configuration or not targeted by this command's scope.\n", argService)
			}
		}
		// If args are provided, we only iterate those that are also in the config.
		// So, fill servicesToDisplay with elements from args that are in availableServicesFromConfig
		for _, argService := range args {
			if availableServicesFromConfig[argService] {
				servicesToDisplay = append(servicesToDisplay, argService)
			}
		}
		if len(servicesToDisplay) == 0 && len(args) > 0 {
		    // All services specified in args were not in config.
		    // Let docker compose ps handle this; it will likely show nothing or error if those services don't exist.
		    // We can add a message here.
		    // fmt.Println("Info: None of the specified services are defined in the upctl.yaml configuration.")
		    // If CaptureCommand returned an error because docker compose ps found no such services, that's already printed.
		    // If it returned empty JSON, our loop won't run.
		    // The current logic is fine: if `servicesToDisplay` is empty, nothing is printed.
		}


	} else {
		for name := range availableServicesFromConfig {
			servicesToDisplay = append(servicesToDisplay, name)
		}
	}
	sort.Strings(servicesToDisplay)


	for _, serviceName := range servicesToDisplay {
		details, isRunning := runningServicesDetails[serviceName]
		statusString := "Not Running"
		if isRunning {
			statusString = "Running"
		}

		if isRunning {
			var portStrings []string
			for _, p := range details.Publishers {
				// Ensure URL is not empty for better display, default to 0.0.0.0 if common
				url := p.URL
				if url == "" || url == "::" { // Docker might use "::" for IPv6 all interfaces
					url = "0.0.0.0"
				}
				portStrings = append(portStrings, fmt.Sprintf("%s:%d->%d/%s", url, p.PublishedPort, p.TargetPort, p.Protocol))
			}
			portsStr := strings.Join(portStrings, ", ")
			if len(portsStr) == 0 { portsStr = "-" }

			commandStr := details.Command
			if len(commandStr) > 20 { commandStr = commandStr[:17] + "..." }
			if commandStr == "" { commandStr = "-" }

			imageStr := details.Image
			if len(imageStr) > 24 { imageStr = imageStr[:21] + "..."}


			psState := details.State
			if psState == "" { psState = details.Status }
			if len(psState) > 17 { psState = psState[:14] + "..."}


			fmt.Printf("%-16s %-14s %-16s %-26s %-24s %-17s %-19s %s\n",
				serviceName, statusString, details.Name, imageStr, commandStr,
				details.Service, psState, portsStr,
			)
		} else {
			// This serviceName is from servicesToDisplay, which means:
			// 1. No args were given, so it's a service from config, and it's not running.
			// 2. Args were given, this serviceName was in args AND in config, but it's not running.
			fmt.Printf("%-16s %-14s %-16s %-26s %-24s %-17s %-19s %s\n",
				serviceName, statusString, "-", "-", "-", "-", "-", "-",
			)
		}
	}
}


// RunDockerComposeUp starts docker compose services. It's public so it can be called from other packages.
func RunDockerComposeUp(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	fmt.Println("Starting Docker Compose services...")
	allServices, _ := cmd.Flags().GetBool("all")
	var composeArgs []string
	composeArgs = append(composeArgs, "compose", "-f", tempComposePath, "up", "-d")
	if !allServices && len(args) > 0 {
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

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	fmt.Println("Stopping Docker Compose services...")
	allServices, _ := cmd.Flags().GetBool("all")
	composeArgs := []string{"compose", "-f", tempComposePath, "down"}
	if !allServices && len(args) > 0 {
		composeArgs = append(composeArgs, args[0])
	}

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

	installAll, _ := cmd.Flags().GetBool("all")
	if !(len(args) > 0 || installAll) {
		fmt.Println("Please provide a service name or --all flag")
		os.Exit(1)
	}

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	err = viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		fmt.Printf("Error loading docker compose config: %s\n", err.Error())
		os.Exit(1)
	}

	if len(args) > 0 {
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
	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	allServicesLogs, _ := cmd.Flags().GetBool("all")
	var logArgs []string
	logArgs = append(logArgs, "compose", "-f", tempComposePath, "logs", "--follow")
	if !allServicesLogs && len(args) > 0 {
		logArgs = append(logArgs, args[0])
	}

	err = ExecuteCommand("docker", logArgs...)
	if err != nil {
		fmt.Printf("Error showing logs: %s\n", err.Error())
		os.Exit(1)
	}
}

// RunDockerImportDB handles importing a database into a Docker MySQL container.
func RunDockerImportDB(cmd *cobra.Command, args []string) {
	progress.Start()
	defer progress.Stop()

	tempComposePath, err := createTempComposeFile()
	if err != nil {
		fmt.Printf("Error creating temporary compose file: %s\n", err.Error())
		os.Exit(1)
	}
	defer os.Remove(tempComposePath)

	fmt.Println("Ensuring MySQL service is running...")
	err = ExecuteCommand("docker", "compose", "-f", tempComposePath, "up", "-d", "mysql")
	if err != nil {
		fmt.Printf("Error starting MySQL service: %s\n", err.Error())
		os.Exit(1)
	}

	dbFilePath := cleanPath(mysqlConfig.DBFile)
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		fmt.Println("Downloading database...")
		path, errPath := exec.LookPath("tsh")
		if errPath != nil {
			fmt.Println("Error finding tsh:", errPath)
			os.Exit(1)
		}
		fmt.Println("Authenticating with Teleport...")
		if errAuth := ExecuteCommand(path, "login", fmt.Sprintf("--proxy=%s", teleportConfig.Host)); errAuth != nil {
			fmt.Println("Error authenticating with Teleport:", errAuth)
			os.Exit(2)
		}
		fmt.Println("Authenticating with AWS...")
		if errAwsAuth := ExecuteCommand(path, "apps", "login", teleportConfig.AWSApp, "--aws-role", teleportConfig.AWSRole); errAwsAuth != nil {
			fmt.Println("Error authenticating with AWS:", errAwsAuth)
			os.Exit(2)
		}
		if errS3 := ExecuteCommand("tsh", "aws", "--app", teleportConfig.AWSApp, "s3", "cp",
			fmt.Sprintf("s3://%s/%s", mysqlConfig.S3Bucket, mysqlConfig.S3Key), dbFilePath,
			"--region", mysqlConfig.S3Region); errS3 != nil {
			fmt.Println("Error downloading database:", errS3)
			os.Exit(2)
		}
	}

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

	fmt.Println("Copying database file to container...")
	tmpPath := "/tmp/import.sql"
	err = ExecuteCommand("docker", "cp", dbFilePath, containerID+":"+tmpPath)
	if err != nil {
		fmt.Printf("Error copying database file to container: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Importing database...")
	importCmdStr := fmt.Sprintf("mysql -u %s -p%s %s < %s",
		mysqlConfig.User, mysqlConfig.Password, mysqlConfig.Database, tmpPath)
	err = ExecuteCommand("docker", "exec", containerID, "bash", "-c", importCmdStr)
	if err != nil {
		fmt.Printf("Error importing database: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Cleaning up...")
	err = ExecuteCommand("docker", "exec", containerID, "rm", tmpPath)
	if err != nil {
		fmt.Printf("Error cleaning up: %s\n", err.Error())
	}
	fmt.Println("Database imported successfully")
}

func createTempComposeFile() (string, error) {
	err := viper.Unmarshal(&dockerComposeConfig)
	if err != nil {
		if dockerComposeConfig.Services == nil { dockerComposeConfig.Services = make(map[string]interface{}) }
		if dockerComposeConfig.Volumes == nil { dockerComposeConfig.Volumes = make(map[string]interface{}) }
		if dockerComposeConfig.Networks == nil { dockerComposeConfig.Networks = make(map[string]interface{}) }
	}

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

	tempFile, err := os.CreateTemp("", "docker-compose-*.yml")
	if err != nil {
		return "", fmt.Errorf("error creating temporary file: %s", err.Error())
	}
	// It's important to close the file descriptor, but do it after writing.
	// The path is still valid after closing.

	yamlData, err := yaml.Marshal(dockerComposeConfig)
	if err != nil {
		tempFile.Close() // Close before returning on error
		return "", fmt.Errorf("error marshaling config to YAML: %s", err.Error())
	}

	if _, err := tempFile.Write(yamlData); err != nil {
		tempFile.Close() // Close before returning on error
		return "", fmt.Errorf("error writing to temporary file: %s", err.Error())
	}

	filePath := tempFile.Name() // Get the name before closing
	if err := tempFile.Close(); err != nil { // Now close
		// Log or handle error, but filePath is still valid for defer os.Remove
		fmt.Fprintf(os.Stderr, "Warning: error closing temporary compose file '%s': %v\n", filePath, err)
	}

	return filePath, nil
}
