package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Manage Docker volumes",
	Long:  `Provides commands to list and remove Docker volumes.`,
}

var volumesLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List Docker volumes",
	Long:  `Lists all Docker volumes. Equivalent to 'docker volume ls'.`,
	Run: func(cmd *cobra.Command, args []string) {
		if progress == nil {
			// Initialize progress spinner if not already (e.g. if called directly)
			// This is a basic initialization, consider using the one from root.go if more complex setup is needed
			// progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
		}
		// progress.Start() // Assuming progress is initialized and available
		// defer progress.Stop()

		fmt.Println("Listing Docker volumes...")
		err := ExecuteCommand("docker", "volume", "ls")
		if err != nil {
			fmt.Printf("Error listing Docker volumes: %s\n", err.Error())
			os.Exit(1)
		}
	},
}

var volumesRmCmd = &cobra.Command{
	Use:   "rm [volume_name...]",
	Short: "Remove Docker volumes",
	Long:  `Removes one or more specified Docker volumes. Equivalent to 'docker volume rm <volume_name>'.`,
	Args:  cobra.MinimumNArgs(1), // Require at least one volume name
	Run: func(cmd *cobra.Command, args []string) {
		if progress == nil {
			// Basic initialization, see comment in lsCmd
			// progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
		}
		// progress.Start()
		// defer progress.Stop()

		fmt.Printf("Removing Docker volume(s): %s\n", strings.Join(args, ", "))
		dockerArgs := append([]string{"volume", "rm"}, args...)
		err := ExecuteCommand("docker", dockerArgs...)
		if err != nil {
			fmt.Printf("Error removing Docker volumes: %s\n", err.Error())
			// os.Exit(1) // Decide if failure to remove one volume should stop everything or report and continue
		} else {
			fmt.Println("Successfully removed volume(s):", strings.Join(args, ", "))
		}
	},
}

func init() {
	volumesCmd.AddCommand(volumesLsCmd)
	volumesCmd.AddCommand(volumesRmCmd)
	// rootCmd.AddCommand(volumesCmd) // This will be done in root.go
}
