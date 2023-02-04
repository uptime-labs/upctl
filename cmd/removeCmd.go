package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [package]",
	Short: "Remove a package",
	Long:  `Remove a installed package from the cluster using Helm. The package argument is required.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var pkg *Package
		if len(args) > 0 {
			for _, p := range packages {
				if p.Name == args[0] {
					pkg = &p
					break
				}
			}
			if pkg == nil {
				fmt.Println("Package not found:", args[0])
				os.Exit(1)
			}
		} else {
			fmt.Println("Please provide a package name")
			for _, p := range packages {
				fmt.Println("  ", p.Name)
			}
			os.Exit(1)
		}

		progress.Start()
		defer progress.Stop()

		// Remove the Helm package
		if err := helmClient.UninstallReleaseByName(pkg.Name); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}

		fmt.Println("Package removed successfully")
	},
}
