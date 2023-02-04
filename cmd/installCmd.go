package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/repo"
)

var installCmd = &cobra.Command{
	Use:   "install [package]",
	Short: "Install a package",
	Long: `Install a package using Helm. The package argument is optional.
	provide --all to install all packages`,
	Args: cobra.MinimumNArgs(0),
	PreRun: func(cmd *cobra.Command, args []string) {
		if !(len(args) > 0 || cmd.Flag("all").Changed) {
			fmt.Println("Please provide a package name or --all")
			os.Exit(1)
		}

		progress.Start()
		defer progress.Stop()
		fmt.Println("Adding repositories...")

		for _, r := range repositories {
			addRepository(r.Name, r.URL)
			progress.Restart()
		}
	},
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
				return
			}
		}

		progress.Start()
		defer progress.Stop()

		if pkg != nil {
			fmt.Println("\nInstalling package...")
			installPkg(pkg)
		} else if cmd.Flag("all").Changed {
			fmt.Println("\nInstalling all packages...")
			for _, pkg := range packages {
				installPkg(&pkg)
				progress.Restart()
			}
		}
	},
}

func installPkg(pkg *Package) {
	name := pkg.Name
	repo := pkg.Repo
	namespace := pkg.Namespace

	overridePath := path.Join(cleanPath(overrides), pkg.Override)
	fmt.Printf("\nInstalling package %s from repo %s with namespace %s and override %s\n", name, repo, namespace, overridePath)

	values, err := ioutil.ReadFile(overridePath)
	if err != nil {
		fmt.Errorf("Error reading override file %s: %s", overridePath, err.Error())
		os.Exit(1)
	}

	// Run the command to install the package with the specified values
	chartSpec := helmclient.ChartSpec{
		ReleaseName: name,
		ChartName:   repo,
		Namespace:   namespace,
		UpgradeCRDs: true,
		Wait:        true,
		ValuesYaml:  string(values),
		Timeout:     time.Duration(1) * time.Minute,
	}

	// Install a chart release.
	// Note that helmclient.Options.Namespace should ideally match the namespace in chartSpec.Namespace.
	if _, err := helmClient.InstallOrUpgradeChart(context.Background(), &chartSpec, nil); err != nil {
		fmt.Errorf("Error installing package %s: %s", name, err.Error())
		os.Exit(2)
	}
}

func addRepository(name string, url string) {
	fmt.Printf(" %s with URL %s\n", name, url)
	// Run the command to add the repository
	// Add a chart-repository to the client.

	if err := helmClient.AddOrUpdateChartRepo(repo.Entry{
		Name: name,
		URL:  url,
	}); err != nil {
		fmt.Errorf("Error adding repo %s: %s", name, err.Error())
		os.Exit(2)
	}
}
