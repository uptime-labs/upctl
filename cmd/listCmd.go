package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all packages",
	Long:  `List all packages`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Listing packages...")
		progress.Start()
		defer progress.Stop()

		namespaces := []string{"default"}

		for _, pkg := range packages {
			// create a unique list of namespaces
			if !contains(namespaces, pkg.Namespace) {
				namespaces = append(namespaces, pkg.Namespace)
			}
		}

		// create a helm client for each namespace
		for _, namespace := range namespaces {
			progress.Restart()
			client := createHelmClient(namespace)

			releases, err := client.ListDeployedReleases()
			if err != nil {
				fmt.Println("Error listing packages:", err)
				progress.Stop()
				os.Exit(1)
			}

			if len(releases) == 0 {
				continue
			}

			for _, r := range releases {
				progress.Restart()
				fmt.Println(" ", r.Name, "->", r.Namespace, "=", r.Chart.Metadata.Version)
			}
		}
	},
}
