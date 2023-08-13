package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/**
Configure the helm repositories command
*/

var configCmd = &cobra.Command{
	Use:   "config [command]",
	Short: "Execute a configuration command",
	Long: `Execute a configuration command. 

Valid commands are: repo, docker

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
		if args[0] == "repo" || args[0] == "repositories" || args[0] == "r" {
			fmt.Println("Adding repositories...")

			for _, r := range repositories {
				progress.Restart()
				// decode the password base64
				if r.Password != "" {
					pwd, _ := base64.StdEncoding.DecodeString(r.Password)
					r.Password = string(pwd)
				}
				addRepository(r.Name, r.URL, r.Username, r.Password)
			}
		} else if args[0] == "docker" || args[0] == "d" {
			configDocker()
		} else {
			fmt.Println("Please provide a valid configuration command")
			fmt.Println("Valid commands are: repo, docker")
			os.Exit(1)
		}
	},
}

// move case docker to a separate function
func configDocker() {
	// create the kubernetes clientset
	clientset, err := createClientSet()
	if err != nil {
		fmt.Println("Error creating kubernetes client config:", err)
		progress.Stop()
		os.Exit(1)
	}

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

	// loop through the namespaces and create the secrets
	for _, r := range dockerConfig.Namespaces {
		progress.Restart()
		fmt.Println("Adding secret for namespace", r)

		err := createNamespace(context.Background(), r)
		if errors.IsAlreadyExists(err) {
			// do nothing
		} else if err != nil {
			fmt.Println("Error creating namespace:", err)
			progress.Stop()
			os.Exit(1)
		}

		// create a secret
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dockerConfig.Name,
				Namespace: r,
				Labels: map[string]string{
					"app": "upctl",
				},
			},
			Type: v1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				v1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{"%s":{"username":"%s","password":"%s","auth":"%s"}}}`, dockerConfig.Registry, dockerConfig.Username, password, base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", dockerConfig.Username, password))))),
			},
		}
		// pass the context and secret to the clientset
		_, err = clientset.CoreV1().Secrets(r).Create(context.Background(), secret, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			// update the secret
			_, err = clientset.CoreV1().Secrets(r).Update(context.Background(), secret, metav1.UpdateOptions{})
			if err != nil {
				fmt.Println("Error updating secret:", err)
				progress.Stop()
				os.Exit(1)
			}
		} else if err != nil {
			fmt.Println("Error creating secret:", err)
			progress.Stop()
			os.Exit(1)
		}
	}

	fmt.Println("Secrets created successfully")
}
