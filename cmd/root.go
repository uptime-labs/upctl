package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/briandowns/spinner"
	helm "github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config is the struct that holds the Repository config values
type Repository struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

// Package is the struct that holds the Package config values
type Package struct {
	Name      string `mapstructure:"name"`
	Repo      string `mapstructure:"repo"`
	Namespace string `mapstructure:"namespace"`
	Override  string `mapstructure:"override"`
}

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

type TeleportConfig struct {
	Host      string `mapstructure:"host"`
	AWSApp    string `mapstructure:"aws_app"`
	AWSRole   string `mapstructure:"aws_role"`
	AWSRegion string `mapstructure:"aws_region"`
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
	overrides      string
	kubeContext    string
	kubeConfig     string
	teleportHost   string

	// HelmClient is the Helm client
	helmClient helm.Client

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

	// Add subcommands flags
	installCmd.Flags().BoolP("all", "a", false, "Install all packages")

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(importDBCmd)
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
	err := viper.UnmarshalKey("repositories", &repositories)
	if err != nil {
		fmt.Printf("Error unmarshaling repositories: %s", err.Error())
		os.Exit(1)
	}

	err = viper.UnmarshalKey("packages", &packages)
	if err != nil {
		fmt.Errorf("Error unmarshaling packages: %s", err.Error())
		os.Exit(1)
	}

	err = viper.UnmarshalKey("teleport", &teleportConfig)
	if err != nil {
		fmt.Errorf("Error unmarshaling teleport: %s", err.Error())
		os.Exit(1)
	}

	err = viper.UnmarshalKey("mysql", &mysqlConfig)
	if err != nil {
		fmt.Errorf("Error unmarshaling mysql: %s", err.Error())
		os.Exit(1)
	}

	overrides = viper.GetString("overrides")
	kubeContext = viper.GetString("kube_context")
	kubeConfig = viper.GetString("kube_config")
	teleportHost = viper.GetString("teleport_host")

	absPath := cleanPath(kubeConfig)

	kubeconfigBytes, err := ioutil.ReadFile(absPath)
	if err != nil {
		fmt.Errorf("Error reading kubeconfig file: %s", err.Error())
		os.Exit(1)
	}

	opt := &helm.KubeConfClientOptions{
		Options: &helm.Options{
			Namespace:        "uptimelabs", // Change this to the namespace you wish the client to operate in.
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            false,
			Linting:          false,
			DebugLog: func(format string, v ...interface{}) {
				fmt.Printf(format, v...)
				fmt.Println()
				progress.Restart()
			},
		},
		KubeContext: kubeContext,
		KubeConfig:  kubeconfigBytes,
	}

	// Create a new Helm client.
	client, err := helm.NewClientFromKubeConf(opt, helm.Burst(100), helm.Timeout(10e9))
	if err != nil {
		fmt.Errorf("Error creating Helm client: %s", err.Error())
		os.Exit(1)
	}

	// Set the global Helm client
	helmClient = client

	// Set the global progress spinner
	progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of upctl",
	Long:  `Print the version number of upctl`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.2.2")
	},
}
