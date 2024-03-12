## Local Development Stack

local-stack is a collection of tools that help develop and test UptimeLabs on a local machine.

- [Local Development Stack](#local-development-stack)
- [1. Before you begin](#1-before-you-begin)
  - [1.1 Prerequisites](#11-prerequisites)
- [1.2 Installation](#12-installation)
  - [Linux](#linux)
  - [MacOS](#macos)
- [1.3 clone the repository](#13-clone-the-repository)
- [1.4 Create a configuration file](#14-create-a-configuration-file)
- [1.5 Copy overrides directory to your home directory](#15-copy-overrides-directory-to-your-home-directory)
- [1.6 Login to the teleport server](#16-login-to-the-teleport-server)
- [2. Enable Kubernetes cluster in Docker Desktop](#2-enable-kubernetes-cluster-in-docker-desktop)
- [3. Configuration](#3-configuration)
  - [3.1 Configure helm repositories](#31-configure-helm-repositories)
- [4. Setting up the local environment](#4-setting-up-the-local-environment)
  - [4.1 Install uptimelabs configuration package](#41-install-uptimelabs-configuration-package)
  - [4.2 Install mysql, keycloak, rabbitmq and redis](#42-install-mysql-keycloak-rabbitmq-and-redis)
  - [4.3 Import uptimelabs mysql database](#43-import-uptimelabs-mysql-database)
  - [4.4 Import keycloak realm](#44-import-keycloak-realm)
- [5. Removing pacakges and retrieving secrets](#5-removing-pacakges-and-retrieving-secrets)
  - [5.1 Removing packages](#51-removing-packages)
  - [5.2 Get rabbitmq secrets](#52-get-rabbitmq-secrets)
- [6. Troubleshooting](#6-troubleshooting)


## 1. Before you begin

### 1.1 Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- kubectl
- mysql-client
- awscli
- WSL2 - must on Windows

Read more about installing the dependencies [here](docs/dependencies.md)

## 1.2 Installation

Download the latest release from the [releases page](https://github.com/uptime-labs/local-stack/releases).

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

## 1.3 clone the repository

```bash
git clone --depth 1 git@github.com:uptime-labs/local-stack.git
cd local-stack
```

## 1.4 Create a configuration file

- Copy the sample configuration file `.upctl.yaml` from the config directory of local-stack to your home directory.

```bash
cp config/upctl.yaml ~/.upctl.yaml
```

- Update the configuration file for your environment.

Obtain helm UptimeLabs private repository credentials from the UptimeLabs team and add them to the `.upctl.yaml` file.

```yaml
repositories:
  - name: uptimelabs
    url: https://uptime-labs.github.io/helm-charts
    username: <username>
    password: <password>
```

## 1.5 Copy overrides directory to your home directory

```bash
mkdir ~/.upctl
cp -R config/overrides ~/.upctl/
```

update the overrides property in the `.upctl.yaml` file to point to the overrides directory in your home directory.

## 1.6 Login to the teleport server

```bash
tsh login --proxy=teleport.uptimelabs.io:443
```

## 2. Enable Kubernetes cluster in Docker Desktop

- Open up the settings screen and Navigate to the Kubernetes tab, then check Enable Kubernetes:

<img src="./docs/docker.png" width="500"/>

## 3. Configuration

### 3.1 Configure helm repositories

The repositories are configured in the `.upctl.yaml` file. 

for the Uptimelabs private Helm repository you must configure the username and password in the `.upctl.yaml` file.

Execute the following command to configure:

```bash
upctl config repo
```
If you modify repositories, re-run the command to update the repositories cache.

## 4. Setting up the local environment

### 4.1 Install uptimelabs configuration package

This package contains the configuration for the uptimelabs applications. This package should be installed first.

```bash
upctl install uptimelabs-envs
```

### 4.2 Install mysql, keycloak, rabbitmq and redis

Install KeyCloak and MySQL

```bash
upctl install mysql
upctl install keycloak
```

Install RabbitMQ

```bash
upctl install rabbitmq-operator
upctl install uptimelabs-messaging
```

Install Redis

```bash
upctl install redis
```

### 4.3 Import uptimelabs mysql database

```bash
upctl import-db
```

### 4.4 Import keycloak realm

- To import the keycloak realm navigate to the keycloak admin console [http://localhost:8085](http://localhost:8085)
- Use `admin` and `uptimelabs` as the username and password.
- Click on the `Add realm` button and select the `config/tenants-realm.json` file to import and click on the `Create` button.

## 5. Removing pacakges and retrieving secrets 

### 5.1 Removing packages

```bash
upctl remove <package name>
```

### 5.2 Get rabbitmq secrets

```bash
kubectl get secret rabbitmq-public-default-user -n rabbitmq --context=docker-desktop -o jsonpath='{.data.username}' | base64 -d && echo

kubectl get secret rabbitmq-public-default-user -n rabbitmq --context=docker-desktop -o jsonpath='{.data.password}' | base64 -d && echo
```

## 6. Troubleshooting

6.1 If mysql installation fails after a previous installation, make sure to delete the mysql persistent volume claim before installing again.

```bash
kubectl delete pvc data-mysql-0 -n mysql
```

6.2 If you are seeing the following error when pulling images from ECR, make sure to configure the ECR credentials.

```log
Failed to pull image "300954903401.dkr.ecr.eu-west-1.amazonaws.com/uptimelabs-slack-events:v0.2.38 ││ ": rpc error: code = Unknown desc = Error response from daemon: failed to resolve reference "300954903401.dkr.ecr.eu-west-1.amazonaws.com/uptimelabs-slack-eve ││ nts:v0.2.38": pulling from host 300954903401.dkr.ecr.eu-west-1.amazonaws.com failed with status code [manifests v0.2.38]: 401 Unauthorized 
```

```bash
upctl config docker
```

if that doesn't resolve the issue, check if you have configured the image pull secrets for deployments.
