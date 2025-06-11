package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

/*
Configure the helm repositories command
*/

var configCmd = &cobra.Command{
	Use:   "config [command]",
	Short: "Execute a configuration command",
	Long: `Execute a configuration command. 

Valid commands are: docker

Example: upctl config repo

repo: Configures the helm repositories for the local development environment
repositories are defined in the config.yaml file.

docker: Configures the ECR image pull secrets for the local development environment
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		progress.Start()
		defer progress.Stop()
		// check for subcommand
		if args[0] == "docker" || args[0] == "d" {
			configDocker()
		} else {
			fmt.Println("Please provide a valid configuration command")
			fmt.Println("Valid commands are: docker")
			os.Exit(1)
		}
	},
}

// configDocker configures Docker authentication, potentially using ECR/Teleport.
func configDocker() {
	var password string

	if dockerConfig.UseTeleport {
		progress.Restart()
		//lookup the teleport client path
		path, err := exec.LookPath("tsh")
		if err != nil {
			fmt.Println("Error finding tsh:", err)
			progress.Stop()
			os.Exit(1)
		}

		fmt.Printf("Authenticating with AWS App: %s...\n", dockerConfig.AWSApp)
		if err := ExecuteCommand(path, "apps", "login", dockerConfig.AWSApp, "--aws-role", teleportConfig.AWSRole); err != nil {
			fmt.Println("Error authenticating with AWS:", err)
			progress.Stop()
			os.Exit(2)
		}

		// Execute the tsh aws ecr login command
		password, err = tshAwsEcrLogin()
		if err != nil {
			fmt.Println(err)
			progress.Stop()
			os.Exit(1)
		}
	} else {
		// get the password from the user
		password = dockerConfig.Password
	}

	// Configure local Docker authentication
	fmt.Println("Configuring Docker authentication...")
	authCmd := fmt.Sprintf("docker login -u %s -p %s %s",
		dockerConfig.Username, password, dockerConfig.Registry)

	// Run the docker login command
	if err := ExecuteCommand("sh", "-c", authCmd); err != nil {
		fmt.Println("Error configuring Docker authentication:", err)
		progress.Stop()
		os.Exit(1)
	}

	fmt.Println("Docker authentication configured successfully")
}
