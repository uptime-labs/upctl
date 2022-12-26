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
			progress.Stop()
			os.Exit(1)
		}

		fmt.Println("Authenticating with Teleport...")
		if err := ExecuteCommand(path, "login", fmt.Sprintf("--proxy=%s", teleportConfig.Host)); err != nil {
			fmt.Println("Error authenticating with Teleport:", err)
			progress.Stop()
			os.Exit(2)
		}

		fmt.Println("Authenticating with AWS...")
		if err := ExecuteCommand(path, "apps", "login", teleportConfig.AWSApp, "--aws-role", teleportConfig.AWSRole); err != nil {
			fmt.Println("Error authenticating with AWS:", err)
			progress.Stop()
			os.Exit(2)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		progress.Start()
		defer progress.Stop()

		dbFilePah := cleanPath(mysqlConfig.DBFile)
		// If file does not exist, download from s3 bucket
		if _, err := os.Stat(dbFilePah); os.IsNotExist(err) {
			// download database from s3 bucket using tsh aws command
			// tsh aws s3 cp s3://<bucket>/<database> <database>
			fmt.Println("Downloading database...")
			fmt.Println(mysqlConfig.S3Bucket, mysqlConfig.S3Key, dbFilePah, mysqlConfig.S3Region)

			if err := ExecuteCommand("tsh", "aws", "--app", teleportConfig.AWSApp, "s3", "cp",
				fmt.Sprintf("s3://%s/%s", mysqlConfig.S3Bucket, mysqlConfig.S3Key), dbFilePah,
				"--region", mysqlConfig.S3Region); err != nil {
				fmt.Println("Error downloading database:", err)
				progress.Stop()
				os.Exit(2)
			}
		}

		fmt.Println("Importing database...")
		path, err := exec.LookPath("mysql")
		if err != nil {
			fmt.Println("Error finding mysql:", err)
			progress.Stop()
			os.Exit(1)
		}

		// execute mysql command to import database from file using mysqlConfig values
		// mysql -h <host> -u <user> -p <password> < <database>
		if err := ExecuteCommand(path, "-h", mysqlConfig.Host, "-P", mysqlConfig.Port, "-u", mysqlConfig.User,
			fmt.Sprintf("-p%s", mysqlConfig.Password), mysqlConfig.Database, "-e", fmt.Sprint("source ", dbFilePah)); err != nil {
			fmt.Println("Error importing database:", err)
			progress.Stop()
			os.Exit(2)
		}
	},
}
