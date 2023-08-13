package cmd

import (
	"fmt"
	"os"
	"time"

	helm "github.com/mittwald/go-helm-client"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	restclient "k8s.io/client-go/rest"
)

// Repository Config is the struct that holds the Repository config values
type Repository struct {
	Name     string `mapstructure:"name"`
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// Package is the struct that holds the Package config values
type Package struct {
	Name      string `mapstructure:"name"`
	Repo      string `mapstructure:"repo"`
	Namespace string `mapstructure:"namespace"`
	Override  string `mapstructure:"override"`
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

	repositories   []Repository
	packages       []Package
	mysqlConfig    MySQLConfig
	teleportConfig TeleportConfig
	dockerConfig   DockerConfig
	overrides      string
	kubeContext    string
	kubeConfigFile string
	teleportHost   string

	progress *spinner.Spinner

	kubeconfigBytes []byte

	restConfig *restclient.Config
	helmClient *helm.Client
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

	// Add subcommands flags
	installCmd.Flags().BoolP("all", "a", false, "Install all packages")

	rootCmd.AddCommand(installCmd, removeCmd, versionCmd,
		importDBCmd, configCmd, listCmd)
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

	// Set default values
	viper.SetDefault("overrides", "./overrides")
	viper.SetDefault("kube_context", "docker-desktop")
	viper.SetDefault("kube_config", "~/.kube/config")

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		fmt.Println("Error reading config file:", err.Error())
		os.Exit(1)
	}

	// Set Viper values to local variables
	err := viper.UnmarshalKey("repositories", &repositories)
	if err != nil {
		fmt.Printf("Error unmarshaling repositories: %s", err.Error())
		os.Exit(1)
	}

	err = viper.UnmarshalKey("packages", &packages)
	if err != nil {
		fmt.Printf("Error unmarshaling packages: %s", err.Error())
		os.Exit(1)
	}

	err = viper.UnmarshalKey("teleport", &teleportConfig)
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

	overrides = viper.GetString("overrides")
	kubeContext = viper.GetString("kube_context")
	kubeConfigFile = viper.GetString("kube_config")
	teleportHost = viper.GetString("teleport_host")

	absPath := cleanPath(kubeConfigFile)

	kubeconfigBytes, err = os.ReadFile(absPath)
	if err != nil {
		fmt.Printf("Error reading kubeconfig file: %s\n", err.Error())
		os.Exit(1)
	}

	kubeConfig, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		fmt.Printf("Error loading kubeconfig file: %s\n", err.Error())
		os.Exit(1)
	}
	// Set the current context to the one specified in the config
	kubeConfig.CurrentContext = kubeContext

	restConfig, err = clientcmd.NewDefaultClientConfig(*kubeConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Set the global progress spinner
	progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of upctl",
	Long:  `Print the version number of upctl`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.4.0")
	},
}

func StopProgress() {
	progress.Stop()
}
