package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var importDBCmd = &cobra.Command{
	Use:   "import-db",
	Short: "Import a database",
	Long:  `Import a database using tsh and mysql.`,
	Args:  cobra.MinimumNArgs(0),
	PreRun: func(cmd *cobra.Command, args []string) {
		progress.Start()
		defer progress.Stop()

		//lookup the teleport client path
		path, err := exec.LookPath("tsh")
		if err != nil {
			fmt.Println("Error finding tsh:", err)
			os.Exit(1)
		}

		fmt.Println("Authenticating with Teleport...")
		if err := ExecuteCommand(path, "login", fmt.Sprintf("--proxy=%s", teleportConfig.Host)); err != nil {
			fmt.Println("Error authenticating with Teleport:", err)
			os.Exit(2)
		}

		fmt.Println("Authenticating with AWS...")
		if err := ExecuteCommand(path, "apps", "login", teleportConfig.AWSApp, "--aws-role", teleportConfig.AWSRole); err != nil {
			fmt.Println("Error authenticating with AWS:", err)
			os.Exit(2)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		progress.Start()
		defer progress.Stop()

		// If file does not exist, download from s3 bucket
		if _, err := os.Stat(mysqlConfig.DBFile); os.IsNotExist(err) {
			// download database from s3 bucket using tsh aws command
			// tsh aws s3 cp s3://<bucket>/<database> <database>
			fmt.Println("Downloading database...")
			fmt.Println(mysqlConfig.S3Bucket, mysqlConfig.S3Key, mysqlConfig.DBFile, mysqlConfig.S3Region)

			if err := ExecuteCommand("tsh", "aws", "s3", "cp",
				fmt.Sprintf("s3://%s/%s", mysqlConfig.S3Bucket, mysqlConfig.S3Key), mysqlConfig.DBFile,
				"--region", mysqlConfig.S3Region); err != nil {
				fmt.Println("Error downloading database:", err)
				os.Exit(2)
			}
		}

		fmt.Println("Importing database...")
		path, err := exec.LookPath("mysql")
		if err != nil {
			fmt.Println("Error finding mysql:", err)
			os.Exit(1)
		}

		// execute mysql command to import database from file using mysqlConfig values
		// mysql -h <host> -u <user> -p <password> < <database>
		if err := ExecuteCommand(path, "-h", mysqlConfig.Host, "-P", mysqlConfig.Port, "-u", mysqlConfig.User,
			fmt.Sprintf("-p%s", mysqlConfig.Password), mysqlConfig.Database, "-e", fmt.Sprint("source ", mysqlConfig.DBFile)); err != nil {
			fmt.Println("Error importing database:", err)
			os.Exit(2)
		}
	},
}
