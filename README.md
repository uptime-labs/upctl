## upctl

upctl is a tool for setting up local development environments using Kubernetes (via Helm) or Docker Compose

- [1. Dependencies](#1-dependencies)
  - [1.1 Software](#11-software)
- [2. Installation](#2-installation)
  - [2.1 Install upctl](#21-install-upctl)
    - [Linux](#linux)
    - [MacOS](#macos)
- [3. Create a configuration file](#3-create-a-configuration-file)
  - [3.1 Copy the configuration file and overrides files to home](#31-copy-the-configuration-file-and-overrides-files-to-home)
  - [3.2 Update the configuration file for your environment.](#32-update-the-configuration-file-for-your-environment)
- [4. Login to the teleport server](#4-login-to-the-teleport-server)
- [5. Configure repository cache](#5-configure-repository-cache)
- [6. Using with Kubernetes (default)](#6-using-with-kubernetes-default)
  - [6.1 Install helm packages](#61-install-helm-packages)
  - [6.2 Import database](#62-import-database)
  - [6.3 Removing packages](#63-removing-packages)
- [7. Using with Docker Compose](#7-using-with-docker-compose)
  - [7.1 Docker Compose Configuration](#71-docker-compose-configuration)
  - [7.2 Install services with Docker Compose](#72-install-services-with-docker-compose)
  - [7.3 Docker Compose Commands](#73-docker-compose-commands)
  - [7.4 Import database with Docker Compose](#74-import-database-with-docker-compose)

# 1. Dependencies

## 1.1 Software
For Kubernetes mode:
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) or [colima](https://github.com/abiosoft/colima)
- kubectl
- mysql-client
- awscli

For Docker Compose mode:
- Docker Engine
- Docker Compose
- mysql-client (if using database imports)
- awscli (if downloading database dumps from S3)

# 2. Installation

## 2.1 Install upctl

### Using Homebrew (macOS and Linux)

```bash
# Add the tap
brew tap uptime-labs/upctl

# Install upctl
brew install upctl
```

### Easy Linux Installation Script (All Distributions)

```bash
curl -fsSL https://raw.githubusercontent.com/uptime-labs/upctl/main/scripts/install.sh | bash
```

Alternatively, you can specify a particular version:

```bash
VERSION=0.8.0 curl -fsSL https://raw.githubusercontent.com/uptime-labs/upctl/main/scripts/install.sh | bash
```

### Manual Installation

Visit the [releases page](https://github.com/uptime-labs/upctl/releases) and download the `upctl` version for your operating system (e.g., Linux, macOS).

Make sure to download the correct version for your operating system.
 - `upctl_0.x.x_linux_amd64` for Intel-based GNU/Linux
 - `upctl_0.x.x_linux_arm64` for Arm-based GNU/Linux
 - `upctl_0.x.x_darwin_amd64` for intel-based macOS
 - `upctl_0.x.x_darwin_arm64` for Arm-based macOS

#### Linux

```bash
cd ~/Downloads
sudo install -o root -g root -m 0755 upctl_0.8.0_linux_amd64 /usr/local/bin/upctl
```

#### MacOS

```bash
cd ~/Downloads
chmod +x upctl_0.8.0_darwin_amd64
sudo mv upctl_0.8.0_darwin_amd64 /usr/local/bin/upctl
```

# 3. Create a configuration file

## 3.1 Copy the configuration file and overrides files to home

```bash
cp config/upctl.yaml ~/.upctl.yaml
cp -R config/overrides ~/.upctl/
```

## 3.2 Update the configuration file for your environment.

The configuration file supports both Kubernetes/Helm and Docker Compose configurations.

```yaml
repositories:
  - name: grafana
    url: https://grafana.github.io/helm-charts
    username: <username>
    password: <password>
```

Repository passwords are optional

# 4. Login to the teleport server

```bash
tsh login --proxy=teleport.example.com
```

# 5. Configure repository cache

For Kubernetes/Helm mode:

```bash
upctl config repo
```
If you modify repositories, re-run the command to update the repositories cache.

# 6. Using with Kubernetes (default)

## 6.1 Install helm packages

```bash
upctl install grafana 
```

To install all packages defined in the configuration:

```bash
upctl install --all
```

## 6.2 Import database 

```bash
upctl import-db
```

## 6.3 Removing packages 

```bash
upctl remove <package name>
```

# 7. Using with Docker Compose

## 7.1 Docker Compose Configuration

Add your Docker Compose services to the `docker_compose` section in your `upctl.yaml`:

```yaml
docker_compose:
  version: '3.8'
  services:
    loki:
      image: grafana/loki:latest
      ports:
        - "3100:3100"
      volumes:
        - loki-data:/loki
      command: -config.file=/etc/loki/local-config.yaml
      restart: unless-stopped
      networks:
        - uptimelabs
    
    grafana:
      image: grafana/grafana:latest
      ports:
        - "3000:3000"
      volumes:
        - grafana-data:/var/lib/grafana
      restart: unless-stopped
      networks:
        - uptimelabs
      environment:
        - GF_SECURITY_ADMIN_USER=admin
        - GF_SECURITY_ADMIN_PASSWORD=admin

  volumes:
    loki-data:
      driver: local
    grafana-data:
      driver: local
      
  networks:
    uptimelabs:
      driver: bridge
```

## 7.2 Install services with Docker Compose

To install packages using Docker Compose instead of Kubernetes/Helm:

```bash
upctl install --docker grafana
```

To install all Docker Compose services:

```bash
upctl install --docker --all
```

## 7.3 Docker Compose Commands

Manage your Docker Compose services:

```bash
# List available services (defined in your upctl.yaml)
upctl list

# Start a specific service (e.g., grafana)
# A service name is required if --all is not used.
upctl up grafana

# Start all services
upctl up --all

# Stop a service (Note: 'down' typically stops all services,
# service-specific stopping might depend on future enhancements or specific flags for 'down')
# The following is an example assuming 'down' might support a service name.
# Check 'upctl down --help' for actual usage.
upctl down grafana
# TODO: Verify 'upctl down' behavior and update docs accordingly. Current focus is 'up'.

# Stop all services
upctl down

# View logs for a specific service (e.g., grafana)
upctl logs grafana

# View logs for all services
upctl logs
# TODO: Verify if 'upctl logs' without args shows all logs or requires a flag.
# Current focus is 'up'.
```

## 7.4 Import database with Docker Compose

Import a database into a Docker MySQL container (ensure the MySQL service is defined in your Docker Compose setup within `upctl.yaml`):

```bash
upctl import-db
```
*Note: The `--docker` flag for `import-db` might be legacy if the global context determines Docker Compose usage. Verify with `upctl import-db --help`.*
*TODO: Clarify if `--docker` is still needed for `import-db` or if context implies it.*