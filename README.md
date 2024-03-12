## Local Development Stack

local-stack is a collection of tools that help develop and test UptimeLabs on a local machine.

- [1. Prerequisites](#1-prerequisites)
  - [1.1 Software](#11-software)
  - [1.2 - Enable kubernets in docker desktop](#12---enable-kubernets-in-docker-desktop)
- [2. Installation](#2-installation)
  - [2.1 Install upctl](#21-install-upctl)
    - [Linux](#linux)
    - [MacOS](#macos)
- [3. Get the code](#3-get-the-code)
- [4. Create a configuration file](#4-create-a-configuration-file)
  - [4.1 Copy the configuration file and overrides files to home](#41-copy-the-configuration-file-and-overrides-files-to-home)
  - [4.2 Update the configuration file for your environment.](#42-update-the-configuration-file-for-your-environment)
- [5. Login to the teleport server](#5-login-to-the-teleport-server)
- [7. Configure repository cache](#7-configure-repository-cache)
- [8. Setting up the local environment](#8-setting-up-the-local-environment)
  - [8.1 Install Essential Components](#81-install-essential-components)
  - [8.2 Import Data to local uptimelabs database](#82-import-data-to-local-uptimelabs-database)
  - [8.3 Import keycloak realm](#83-import-keycloak-realm)
- [9. Removing pacakges and retrieving secrets](#9-removing-pacakges-and-retrieving-secrets)
  - [9.1 Removing packages](#91-removing-packages)
  - [9.2 Get rabbitmq secrets](#92-get-rabbitmq-secrets)
- [10. Troubleshooting](#10-troubleshooting)


# 1. Prerequisites

## 1.1 Software
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) or [colima](https://github.com/abiosoft/colima)
- kubectl
- mysql-client
- awscli

Read more about installing the dependencies [here](docs/dependencies.md)

## 1.2 - Enable kubernets in docker desktop

- Open up the settings screen and Navigate to the Kubernetes tab, then check Enable Kubernetes:

<img src="./docs/docker.png" width="500"/>

# 2. Installation

## 2.1 Install upctl

Visit [releases page](https://github.com/uptime-labs/local-stack/releases) and download the `upctl` version for your operating system (e.g., Linux, macOS).

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

# 3. Get the code

```bash
git clone --depth 1 git@github.com:uptime-labs/local-stack.git
cd local-stack
```

# 4. Create a configuration file

## 4.1 Copy the configuration file and overrides files to home

```bash
cp config/upctl.yaml ~/.upctl.yaml
cp -R config/overrides ~/.upctl/
```

## 4.2 Update the configuration file for your environment.

Obtain helm UptimeLabs private repository credentials and update configuration `.upctl.yaml` file.

Look for the password and replace it with the password

```yaml
repositories:
  - name: uptimelabs
    url: https://uptime-labs.github.io/helm-charts
    username: <username>
    password: <password>
```

# 5. Login to the teleport server

```bash
tsh login --proxy=teleport.uptimelabs.io:443
```

# 7. Configure repository cache

```bash
upctl config repo
```
If you modify repositories, re-run the command to update the repositories cache.

# 8. Setting up the local environment

## 8.1 Install Essential Components

Execute the commands followed by next

```bash
upctl install uptimelabs-envs
upctl install mysql
upctl install keycloak
upctl install rabbitmq-operator
upctl install uptimelabs-messaging
upctl install redis
```

## 8.2 Import Data to local uptimelabs database

```bash
upctl import-db
```

## 8.3 Import keycloak realm

- To import the keycloak realm navigate to the keycloak admin console [http://localhost:8085](http://localhost:8085)
- Use `admin` and `uptimelabs` as the username and password.
- Click on the `Add realm` button and select the `config/tenants-realm.json` file to import and click on the `Create` button.

# 9. Removing pacakges and retrieving secrets 

## 9.1 Removing packages

```bash
upctl remove <package name>
```

## 9.2 Get rabbitmq secrets

```bash
kubectl get secret rabbitmq-public-default-user -n rabbitmq --context=docker-desktop -o jsonpath='{.data.username}' | base64 -d && echo

kubectl get secret rabbitmq-public-default-user -n rabbitmq --context=docker-desktop -o jsonpath='{.data.password}' | base64 -d && echo
```

# 10. Troubleshooting

If mysql installation fails after a previous installation, make sure to delete the mysql persistent volume claim before installing again.

```bash
kubectl delete pvc data-mysql-0 -n mysql
```

