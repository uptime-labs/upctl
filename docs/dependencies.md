# Installation instructions for dependencies

## kubectl - Kubernetes command-line interface

- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) is a command-line tool for controlling Kubernetes clusters. It allows you to run commands against Kubernetes clusters from the terminal.

### Linux

```bash
$ curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
$ sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
```

### macOS

```bash
brew install kubectl
```

## mysql-client

- [mysql-client](https://dev.mysql.com/doc/refman/8.0/en/mysql.html) is a command-line tool for controlling MySQL databases. It allows you to run commands against MySQL databases from the terminal.

### Linux

```bash
$ sudo apt-get install mysql-client
```

### macOS

```bash
brew install mysql-client
```

## AWS CLI

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) is a command-line tool for controlling AWS services. It allows you to run commands against AWS services from the terminal.

### Linux

```bash
$ curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
$ unzip awscliv2.zip
$ sudo ./aws/install
```

### macOS

```bash
brew install awscli
```

## WSL2 - Windows Subsystem for Linux

- [WSL2](https://docs.microsoft.com/en-us/windows/wsl/install-win10) is a compatibility layer for running Linux binary executables (in ELF format) natively on Windows 10 and Windows Server 2019.

### Windows

```PowerShell
# Enable the Windows Subsystem for Linux
dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart

# Enable Virtual Machine feature
dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart

# Download the Linux kernel update package
Invoke-WebRequest -Uri https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi -OutFile wsl_update_x64.msi

# Install the Linux kernel update package
Start-Process msiexec.exe -Wait -ArgumentList '/i wsl_update_x64.msi /quiet'

# Set WSL 2 as the default version when installing a new Linux distribution
wsl --set-default-version 2
```

Download the latest Ubuntu LTS appx package from the Microsoft Store
