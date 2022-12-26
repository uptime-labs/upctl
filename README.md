## upctl

upctl is a wrapper on helm for easy setup of develpment environments 

- [1. Dependencies](#1-dependencies)
  - [1.1 Software](#11-software)
  - [1.2 - Enable kubernets in docker desktop](#12---enable-kubernets-in-docker-desktop)
- [2. Installation](#2-installation)
  - [2.1 Install upctl](#21-install-upctl)
    - [Linux](#linux)
    - [MacOS](#macos)
- [3. Create a configuration file](#3-create-a-configuration-file)
  - [3.1 Copy the configuration file and overrides files to home](#31-copy-the-configuration-file-and-overrides-files-to-home)
  - [3.2 Update the configuration file for your environment.](#32-update-the-configuration-file-for-your-environment)
- [4. Login to the teleport server](#4-login-to-the-teleport-server)
- [5. Configure repository cache](#5-configure-repository-cache)
- [6. Install helm packages](#6-install-helm-packages)
- [7. Import database](#7-import-database)
- [8. Removing pacakges](#8-removing-pacakges)


# 1. Dependencies

## 1.1 Software
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) or [colima](https://github.com/abiosoft/colima)
- kubectl
- mysql-client
- awscli

## 1.2 - Enable kubernets in docker desktop

- Open up the settings screen and Navigate to the Kubernetes tab, then check Enable Kubernetes:

<img src="./docs/docker.png" width="500"/>

# 2. Installation

## 2.1 Install upctl

Visit [releases page](https://github.com/uptime-labs/upctl/releases) and download the `upctl` version for your operating system (e.g., Linux, macOS).

Make sure to download the correct version for your operating system.
 - `upctl_0.x.x_linux_amd64` for Intel-based GNU/Linux
 - `upctl_0.x.x_linux_arm64` for Arm-based GNU/Linux
 - `upctl_0.x.x_darwin_amd64` for intel-based macOS
 - `upctl_0.x.x_darwin_arm64` for Arm-based macOS

### Linux

```bash
cd ~/Downloads
sudo install -o root -g root -m 0755 upctl_0.8.0_linux_amd64 /usr/local/bin/upctl
```

### MacOS

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

```yaml
repositories:
  - name: grafana
    url: https://grafana.github.io/helm-charts
    username: <username>
    password: <password>
```

repository passwords are optional

# 4. Login to the teleport server

```bash
tsh login --proxy=teleport.example.com
```

# 5. Configure repository cache

```bash
upctl config repo
```
If you modify repositories, re-run the command to update the repositories cache.

# 6. Install helm packages

```bash
upctl install grafana 
```

# 7. Import database 

```bash
upctl import-db
```

# 8. Removing pacakges 

```bash
upctl remove <package name>
```