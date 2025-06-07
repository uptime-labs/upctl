package cmd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DockerComposeConfigForDoctor is a simplified struct for doctor command's needs.
type DockerComposeConfigForDoctor struct {
	Services map[string]interface{} `mapstructure:"services"`
	Volumes  map[string]interface{} `mapstructure:"volumes"`  // Kept for structural integrity if present
	Networks map[string]interface{} `mapstructure:"networks"` // Kept for structural integrity if present
}

// UpctlConfigForValidation defines the expected structure of upctl.yaml for validation.
type UpctlConfigForValidation struct {
	Services       map[string]interface{} `mapstructure:"services"`
	Volumes        map[string]interface{} `mapstructure:"volumes"`
	Networks       map[string]interface{} `mapstructure:"networks"`
	MySQLConfig    MySQLConfig            `mapstructure:"mysql"`
	TeleportConfig TeleportConfig         `mapstructure:"teleport"`
	DockerConfig   DockerConfig           `mapstructure:"docker_config"`
}

// MySQLConfig is the struct that holds the config values
type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Port     string `mapstructure:"port"`
	DBFile   string `mapstructure:"db_file"`
	S3Bucket string `mapstructure:"s3_bucket"`
	S3Key    string `mapstructure:"s3_key"`
	S3Region string `mapstructure:"s3_region"`
}

// TeleportConfig is the struct that holds the Teleport config values
type TeleportConfig struct {
	Host      string `mapstructure:"host"`
	AWSApp    string `mapstructure:"aws_app"`
	AWSRole   string `mapstructure:"aws_role"`
	AWSRegion string `mapstructure:"aws_region"`
}

// DockerConfig is the struct that holds the Docker config values
type DockerConfig struct {
	Name        string   `mapstructure:"name"`
	Namespaces  []string `mapstructure:"namespaces"`
	Registry    string   `mapstructure:"registry"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	UseTeleport bool     `mapstructure:"use_teleport"`
	AWSApp      string   `mapstructure:"aws_app"`
}

var (
	// Used for flags.
	cfgFile string

	rootCmd = &cobra.Command{
		Use:   "upctl",
		Short: "upctl is a CLI tool to manage UpTimeLabs local development environment",
		Long:  `upctl is a CLI tool to manage UpTimeLabs local development environment`,
	}

	mysqlConfig    MySQLConfig
	teleportConfig TeleportConfig
	dockerConfig   DockerConfig
	teleportHost   string

	progress *spinner.Spinner
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.upctl.yaml)")
	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	viper.SetDefault("author", "Gamunu Balagalla <gamunu@upltimelabs.io>")
	viper.SetDefault("license", "(C) UpTimeLabs")

	rootCmd.AddCommand(versionCmd, importDBCmd, configCmd, startCmd, psCmd, doctorCmd, validateCmd)
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the upctl.yaml configuration file",
	Long:  `Checks the syntax and structure of the upctl.yaml file.`,
	Run:   runValidationChecks,
}

func runValidationChecks(cmd *cobra.Command, args []string) {
	fmt.Println("Validating upctl.yaml...")

	// Config file loading check
	configFileUsed := viper.ConfigFileUsed()
	if configFileUsed == "" {
		// This case might be rare if initConfig is always effective.
		// However, if 'upctl validate --config some_nonexistent_file' is run,
		// or if $HOME/.upctl.yaml & ./.upctl.yaml are missing.
		home, _ := os.UserHomeDir() // For error message
		fmt.Printf("Error: Configuration file not found. Searched in %s/.upctl.yaml and ./.upctl.yaml.\n", home)
		fmt.Println("Please ensure a configuration file exists or specify one with --config.")
		return
	}
	fmt.Println("Using configuration file:", configFileUsed)

	// YAML Syntax Check - viper.ReadInConfig() in initConfig already does this.
	// If initConfig failed, it would have os.Exit(1).
	// If we are here, ReadInConfig in initConfig was successful.
	// We can re-read it or rely on viper's cached state.
	// For an explicit validation command, re-reading can be more robust if the file changed
	// or if we want to isolate validation logic, but initConfig already ensures basic readability.
	// The prompt implies viper.ReadInConfig() should be called here again for the check.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error: Failed to read or parse configuration file. Details: %v\n", err)
		return
	}
	fmt.Println("YAML syntax: OK")

	// Structure Validation
	var cfg UpctlConfigForValidation
	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Printf("Error: Configuration file structure is invalid. Ensure top-level keys and their types are correct. Details: %v\n", err)
		return
	}
	fmt.Println("Overall structure: OK")

	// Specific check for 'services'
	if cfg.Services == nil {
		fmt.Println("Error: The 'services' key is missing or empty in upctl.yaml. This is a required field.")
		return
	}
	fmt.Println("'services' key: Present and structurally valid (according to unmarshal).")

	// Potentially check other required sections if any, e.g. mysql, teleport, docker_config
	// For now, just checking their structural validity via Unmarshal.

	fmt.Println("upctl.yaml is valid.")
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check for potential issues with upctl setup and configuration",
	Long:  `Diagnoses potential problems like missing or invalid configuration, and port conflicts.`,
	Run:   runDoctorChecks,
}

func runDoctorChecks(cmd *cobra.Command, args []string) {
	fmt.Println("--- Upctl Doctor ---")

	// Check 1: Config file existence and loading
	fmt.Print("1. Checking config file... ")
	if viper.ConfigFileUsed() == "" {
		fmt.Println("Error: Config file not found. Please ensure .upctl.yaml exists in your home directory or current directory.")
		// Attempt to read anyway, viper might find it if SetConfigName was used, though initConfig should handle this.
		// For doctor, we rely on initConfig having run.
	} else {
		fmt.Printf("OK (using %s)\n", viper.ConfigFileUsed())
	}

	// This check might be redundant if initConfig already exited on error.
	// However, doctor can be an independent check.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("   Error: Config file not found by Viper. Expected $HOME/.upctl.yaml or ./.upctl.yaml.")
		} else {
			fmt.Printf("   Error: Could not read config file: %v. YAML might be invalid.\n", err)
		}
		// Don't return yet, try to proceed with other checks if possible, or make a decision to stop.
		// For port checks, we need the config, so we'll stop here if it's unreadable.
		return
	}

	// Check 2: Validate upctl.yaml structure
	fmt.Print("2. Validating config structure (services, volumes, networks)... ")
	var cfg DockerComposeConfigForDoctor
	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Printf("Error: Could not parse upctl.yaml structure: %v\n", err)
		return // Can't proceed with port checks if structure is wrong
	}
	fmt.Println("OK")

	// Check 3: Services definition
	fmt.Print("3. Checking for 'services' definition... ")
	if cfg.Services == nil {
		fmt.Println("Error: 'services' key not found or empty in upctl.yaml. Cannot check for port conflicts.")
		return
	}
	if len(cfg.Services) == 0 {
		fmt.Println("Info: No services defined under 'services' key in upctl.yaml.")
	} else {
		fmt.Println("OK")
	}

	// Check 4: Port conflicts
	fmt.Println("4. Checking for port conflicts...")
	if cfg.Services != nil && len(cfg.Services) > 0 {
		parsedHostPorts := make(map[string]string) // To detect internal duplicates

		for serviceName, serviceData := range cfg.Services {
			serviceMap, ok := serviceData.(map[string]interface{})
			if !ok {
				fmt.Printf("   Warning: Could not parse service definition for '%s'. Skipping port check.\n", serviceName)
				continue
			}

			portsInterface, exists := serviceMap["ports"]
			if !exists {
				// fmt.Printf("   Info: No ports defined for service '%s'.\n", serviceName)
				continue // No ports defined for this service
			}

			portsList, ok := portsInterface.([]interface{})
			if !ok {
				fmt.Printf("   Warning: 'ports' for service '%s' is not a list. Skipping port check.\n", serviceName)
				continue
			}

			for _, portEntryInterface := range portsList {
				portEntry, ok := portEntryInterface.(string)
				if !ok {
					fmt.Printf("   Warning: Invalid port entry (not a string) for service '%s'. Skipping.\n", serviceName)
					continue
				}

				parts := strings.Split(portEntry, ":")
				var hostPort string
				var hostIP string // Currently not used for listening check, but parsed

				if len(parts) == 1 { // "CONTAINER_PORT" or "HOST_PORT" (docker-compose treats as HOST_PORT:CONTAINER_PORT if CONTAINER_PORT not specified elsewhere)
					hostPort = parts[0]
				} else if len(parts) == 2 { // "HOST_PORT:CONTAINER_PORT"
					hostPort = parts[0]
				} else if len(parts) == 3 { // "IP:HOST_PORT:CONTAINER_PORT"
					hostIP = parts[0]
					hostPort = parts[1]
				} else {
					fmt.Printf("   Warning: Invalid port format '%s' for service '%s'. Skipping.\n", portEntry, serviceName)
					continue
				}

				// Validate if hostPort is a number (it should be, based on parsing logic)
				if _, err := strconv.Atoi(hostPort); err != nil {
					fmt.Printf("   Warning: Host port part '%s' (from entry '%s' for service '%s') is not a valid number. Skipping.\n", hostPort, portEntry, serviceName)
					continue
				}

				listenAddress := ":" + hostPort
				if hostIP != "" {
					listenAddress = hostIP + ":" + hostPort
				}

				// Check for internal conflicts first
				if conflictingService, exists := parsedHostPorts[listenAddress]; exists {
					fmt.Printf("   Error: Port %s (service: %s) conflicts with service '%s' within upctl.yaml.\n", hostPort, serviceName, conflictingService)
					continue // Don't check this port on the host if it's already an internal conflict
				}
				parsedHostPorts[listenAddress] = serviceName

				listener, err := net.Listen("tcp", listenAddress)
				if err != nil {
					fmt.Printf("   Error: Port %s (service: %s, address: %s) is already in use on the host.\n", hostPort, serviceName, listenAddress)
				} else {
					fmt.Printf("   Info: Port %s (service: %s, address: %s) is available.\n", hostPort, serviceName, listenAddress)
					listener.Close()
				}
			}
		}
	} else {
		fmt.Println("   Info: No services with ports to check.")
	}
	fmt.Println("--- Doctor checks complete ---")
}

// psCmd represents the ps command
var psCmd = &cobra.Command{
	Use:   "ps [options]",
	Short: "List running services",
	Long:  `List running services managed by Docker Compose. Accepts docker compose ps flags.`,
	Args:  cobra.ArbitraryArgs,
	Run: func(ccmd *cobra.Command, args []string) {
		if progress == nil {
			// Initialize spinner if it hasn't been
			progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
		}
		RunDockerComposePs(ccmd, args)
	},
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [service]",
	Short: "Start services (equivalent to 'docker up')",
	Long:  `Start specified or all services using Docker Compose.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(ccmd *cobra.Command, args []string) {
		if progress == nil {
			// Initialize spinner if it hasn't been
			// This can happen if initConfig wasn't called (e.g. in tests or specific command flows)
			// For safety, although initConfig should always set it up in normal execution.
			progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
		}
		// Call the public function from the cmd package
		RunDockerComposeUp(ccmd, args)
	},
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".upctl" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".") // optionally look for config in the working directory
		viper.SetConfigType("yaml")
		viper.SetConfigName(".upctl")
	}

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		fmt.Println("Error reading config file:", err.Error())
		os.Exit(1)
	}

	// Set Viper values to local variables
	err := viper.UnmarshalKey("teleport", &teleportConfig)
	if err != nil {
		fmt.Printf("Error unmarshaling teleport: %s", err.Error())
		os.Exit(1)
	}

	err = viper.UnmarshalKey("mysql", &mysqlConfig)
	if err != nil {
		fmt.Printf("Error unmarshaling mysql: %s", err.Error())
		os.Exit(1)
	}

	// unmarshall docker config
	err = viper.UnmarshalKey("docker_config", &dockerConfig)
	if err != nil {
		fmt.Printf("Error unmarshaling docker_config: %s", err.Error())
		os.Exit(1)
	}

	teleportHost = viper.GetString("teleport_host")

	// Set the global progress spinner
	progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of upctl",
	Long:  `Print the version number of upctl`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.5.0 (with Docker Compose support)")
	},
}

func StopProgress() {
	progress.Stop()
}
