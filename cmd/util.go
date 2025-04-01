package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	helm "github.com/mittwald/go-helm-client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// cleanPath cleans up a path and expands ~ to the user's home directory
// if it is present. It then converts the path to an absolute path.
func cleanPath(path string) string {
	cleanedPath := filepath.Clean(path)

	// check backward and forward paths for Windows
	if strings.HasPrefix(cleanedPath, "~/") || strings.HasPrefix(cleanedPath, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting user's home directory: %s", err)
		}
		expandedPath := filepath.Join(homeDir, cleanedPath[2:])
		cleanedPath = expandedPath
	}

	if strings.HasPrefix(cleanedPath, "/tmp") || strings.HasPrefix(cleanedPath, "\\tmp") {
		expandedPath := filepath.Join(os.TempDir(), cleanedPath[2:])
		cleanedPath = expandedPath
	}

	absPath, err := filepath.Abs(cleanedPath)
	if err != nil {
		log.Fatalf("Error converting to absolute path: %s", err)
	}
	return absPath
}

// ExecuteCommandResult represents the result of executing a CLI command.
type ExecuteCommandResult struct {
	Stdout string
	Stderr string
}

// ExecuteCommand executes the given CLI command and streams the output in real time
func ExecuteCommand(command string, args ...string) error {
	var result ExecuteCommandResult

	cmd := exec.Command(command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	outScanner := bufio.NewScanner(stdout)
	errScanner := bufio.NewScanner(stderr)
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		for outScanner.Scan() {
			progress.Restart()
			fmt.Println(outScanner.Text())
			result.Stdout += outScanner.Text() + "\n"
		}
		if err := outScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "error reading stdout:", err)
		}
		close(stdoutDone)
	}()
	go func() {
		for errScanner.Scan() {
			progress.Restart()
			fmt.Println(errScanner.Text())
			result.Stderr += errScanner.Text() + "\n"
		}
		if err := errScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "error reading stderr:", err)
		}
		close(stderrDone)
	}()
	if err := cmd.Wait(); err != nil {
		return err
	}
	<-stdoutDone
	<-stderrDone
	return nil
}

// Create kubernetes clientset
func createClientSet() (*kubernetes.Clientset, error) {
	// Check if Kubernetes config is available
	if restConfig == nil {
		return nil, fmt.Errorf("kubernetes configuration is not available")
	}

	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: kubeContext,
	}

	absPath := cleanPath(kubeConfigFile)
	// get the kubeconfig
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: absPath}
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, overrides)

	// get client config from bytes
	config, err := cfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// create a namespace
func createNamespace(ctx context.Context, name string) error {
	clientset, err := createClientSet()
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// tshAwsEcrLogin
func tshAwsEcrLogin() (string, error) {
	cmd := exec.Command("tsh", "aws", "--app", dockerConfig.AWSApp, "ecr", "get-login-password", "--region", "eu-west-1")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error executing command: %v, stderr: %s", err, errBuf.String())
	}

	scanner := bufio.NewScanner(&outBuf)
	for scanner.Scan() {
		return scanner.Text(), nil
	}
	return "", fmt.Errorf("no password found")
}

// create helm client
func createHelmClient(namespace string) helm.Client {
	// get temporary directory
	tmpDir := os.TempDir()
	opt := &helm.RestConfClientOptions{
		Options: &helm.Options{
			Namespace:        namespace,
			RepositoryCache:  fmt.Sprintf("%s/.helmcache", tmpDir),
			RepositoryConfig: fmt.Sprintf("%s/.helmrepo", tmpDir),
			Debug:            true,
			Linting:          false,
			DebugLog: func(format string, v ...interface{}) {
				progress.Restart()
				fmt.Printf(format, v...)
				fmt.Println()
			},
		},
		RestConfig: restConfig,
	}

	// Create a new Helm client.
	client, err := helm.NewClientFromRestConf(opt)
	if err != nil {
		fmt.Printf("Error creating Helm client: %s", err.Error())
		os.Exit(1)
	}

	return client
}

func contains(elements []string, element string) bool {
	for _, n := range elements {
		if n == element {
			return true
		}
	}
	return false
}
