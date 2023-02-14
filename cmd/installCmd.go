package cmd

import (
	"context"
	"fmt"
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
	Run: func(cmd *cobra.Command, args []string) {
		if !(len(args) > 0 || cmd.Flag("all").Changed) {
			fmt.Println("Please provide a package name or --all")
			os.Exit(1)
		}

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
				fmt.Println()
			}
		}
	},
}

func installPkg(pkg *Package) {
	name := pkg.Name
	r := pkg.Repo
	namespace := pkg.Namespace

	overridePath := path.Join(cleanPath(overrides), pkg.Override)

	var values []byte
	// check if is a file
	info, err := os.Stat(overridePath)
	if err == nil && !info.IsDir() {
		values, err = os.ReadFile(overridePath)
		if err != nil {
			fmt.Printf("error reading override file %s: %s", overridePath, err.Error())
			progress.Stop()
			os.Exit(1)
		}
		fmt.Printf("Package: %s, Repo: %s, Namespace %s, Override: %s\n", name, r, namespace, overridePath)
	} else {
		fmt.Printf("Package: %s, Repo: %s, Namespace: %s\n", name, r, namespace)
	}

	// Run the command to install the package with the specified values
	chartSpec := helmclient.ChartSpec{
		ReleaseName:     name,
		ChartName:       r,
		Namespace:       namespace,
		UpgradeCRDs:     true,
		Wait:            true,
		CreateNamespace: true,
		ValuesYaml:      string(values),
		Timeout:         time.Duration(5) * time.Minute,
	}

	client := createHelmClient(namespace)
	// Install a chart release.
	// Note that helmclient.Options.Namespace should ideally match the namespace in chartSpec.Namespace.
	if _, err := client.InstallOrUpgradeChart(context.Background(), &chartSpec, nil); err != nil {
		fmt.Printf("error installing package %s: %s\n", name, err.Error())
		progress.Stop()
		os.Exit(2)
	}
}

func addRepository(name string, url string, user string, password string) {
	fmt.Printf(" %s with URL %s\n", name, url)
	// Run the command to add the repository
	// Add a chart-repository to the client.
	client := createHelmClient("default")
	if err := client.AddOrUpdateChartRepo(repo.Entry{
		Name:               name,
		URL:                url,
		Username:           user,
		Password:           password,
		PassCredentialsAll: true,
	}); err != nil {
		fmt.Printf("error adding repo %s: %s\n", name, err.Error())
		progress.Stop()
		os.Exit(2)
	}
}
