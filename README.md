## Local Development Stack

local-stack is a collection of tools that help develop and test UptimeLabs on a local machine.

- [Local Development Stack](#local-development-stack)
- [Before you begin](#before-you-begin)
  - [Prerequisites](#prerequisites)
- [1. Installation](#1-installation)
  - [1.1 Download the latest release](#11-download-the-latest-release)
    - [Linux](#linux)
    - [MacOS](#macos)
  - [1.2 clone the repository](#12-clone-the-repository)
  - [1.3 Create a configuration file](#13-create-a-configuration-file)
  - [1.4 Copy overrides directory to your home directory](#14-copy-overrides-directory-to-your-home-directory)
  - [1.5 Login to the teleport server](#15-login-to-the-teleport-server)
- [2. Enable Kubernetes cluster in Docker Desktop](#2-enable-kubernetes-cluster-in-docker-desktop)
- [3. Quick Intro to repositories and packages](#3-quick-intro-to-repositories-and-packages)
- [4. Configuration](#4-configuration)
  - [4.1 Configure helm repositories](#41-configure-helm-repositories)
  - [4.2 Configure ECR credentials](#42-configure-ecr-credentials)
- [5. Setting up the local environment](#5-setting-up-the-local-environment)
  - [5.1 Install uptimelabs configuration package](#51-install-uptimelabs-configuration-package)
  - [5.2 Install mysql, keycloak and rabbitmq](#52-install-mysql-keycloak-and-rabbitmq)
    - [5.2.1 Import uptimelabs mysql database](#521-import-uptimelabs-mysql-database)
    - [5.2.2 Import keycloak realm](#522-import-keycloak-realm)
  - [5.3 Removing packages](#53-removing-packages)
- [6. Troubleshooting](#6-troubleshooting)


## Before you begin

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- kubectl
- mysql-client
- awscli
- WSL2 - must on Windows

Read more about installing the dependencies [here](docs/dependencies.md)

---
## 1. Installation

### 1.1 Download the latest release

Download the latest release from the [releases page](https://github.com/uptime-labs/local-stack/releases).

Make sure to download the correct version for your operating system.
 - `upctl_0.x.x_linux_amd64` for Intel-based GNU/Linux
 - `upctl_0.x.x_linux_arm64` for Arm-based GNU/Linux
 - `upctl_0.x.x_darwin_amd64` for intel-based macOS
 - `upctl_0.x.x_darwin_arm64` for Arm-based macOS


#### Linux

```bash
$ cd ~/Downloads
$ sudo install -o root -g root -m 0755 upctl_0.8.0_linux_amd64 /usr/local/bin/upctl
```

#### MacOS

```bash
$ cd ~/Downloads
$ chmod +x upctl_0.8.0_darwin_amd64
$ sudo mv upctl_0.8.0_darwin_amd64 /usr/local/bin/upctl
```

### 1.2 clone the repository

```bash
$ git clone --depth 1 git@github.com:uptime-labs/local-stack.git
```

### 1.3 Create a configuration file

- Copy the sample configuration file `.upctl.yaml` from the config directory of local-stack to your home directory.

```bash
$ cp config/upctl.yaml ~/.upctl.yaml
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

### 1.4 Copy overrides directory to your home directory

```bash
$ mkdir ~/.upctl
$ cp -R config/overrides ~/.upctl/
```

update the overrides property in the `.upctl.yaml` file to point to the overrides directory in your home directory.

### 1.5 Login to the teleport server

```bash
$ tsh login --proxy=teleport.uptimelabs.io:443
```

---

## 2. Enable Kubernetes cluster in Docker Desktop

- Open up the settings screen and Navigate to the Kubernetes tab, then check Enable Kubernetes:

<img src="./docs/docker.png" width="500"/>

## 3. Quick Intro to repositories and packages

We use helm as a package manager to install packages into the Kubernetes clusters. To simplify the management of repositories and the packages the local-stack include a cli and configuration files.

- **.upctl.yaml**

  - `repositories`

    This property contains a list of required helm repositories to pull helm packages. When you are configuring repositories you can obtain the repository URL from the maintainer usualy from (https://artifacthub.io/) and give any preferred name for the name feild.
    Following is an example of configuring the superset repository, you can give any unique `name` to the name property.

    ```yaml
    - name: superset
      url: https://apache.github.io/superset
    ```

  - `packages`

    This perity contains all the packages that can be configured to be installed into the local Kubernetes cluster. To define a installable package there are several properties.

    - `name` - The name of helm package. This will be used as an installation name and the name will be prepended to the Kubernetes resources created by this helm package. You can give any preferred name for this.
    - `repo` - The combination of repo and package names in format `<repo name>/<package name>`. The package name as defined by the helm package maintainer. Repository name is the name given in the `repositories.yaml`.
    - `namespace` - Namespace of the package resources should install into.
    - `override` - Helm value files to override default helm package values.

    Values for each package can be modified using the overrides config files located in the `<configuration-root>/overrides` directory.
    
    ```yaml
    - name: mosquitto # package name
      repo: k8shome/mosquitto # hem repository name / chart name
      namespace: uptimelabs # namespace to install the package (this will get automatically created)
      override: mqtt.yaml # helm value file for the configuration override
    ```
---
## 4. Configuration

### 4.1 Configure helm repositories

This command will configure the helm repositories for the local Kubernetes cluster. This is required to pull the helm packages from the helm repositories.

The repositories are configured in the `.upctl.yaml` file. You can add or remove repositories from the file and then run the command to configure the repositories.
for the Uptimelabs private Helm repository you must configure the username and password in the `.upctl.yaml` file.

Execute the following command to configure the helm repositories.

```bash
$ upctl config repo
```

### 4.2 Configure ECR credentials

This command will configure the ECR credentials for the local Kubernetes cluster. This is required to pull the images from the private ECR repository.

```bash
$ upctl config docker
```
---
## 5. Setting up the local environment

### 5.1 Install uptimelabs configuration package

This package contains the configuration for the uptimelabs applications. This package should be installed first.

```bash
$ upctl install uptimelabs-envs
```

### 5.2 Install mysql, keycloak and rabbitmq

Install keycloak and mysql packages. This will install the keycloak and mysql into the local Kubernetes cluster.

```bash
$ upctl install mysql
$ upctl install keycloak
```

Install rabbitmq-operator and uptimelabs-messaging packages. This will install the rabbitmq-operator and rabbitmq cluster into the local Kubernetes cluster.

```bash
$ upctl install rabbitmq-operator
$ upctl install uptimelabs-messaging
```

#### 5.2.1 Import uptimelabs mysql database

```bash
$ upctl import-db
```

#### 5.2.2 Import keycloak realm

- To import the keycloak realm navigate to the keycloak admin console [http://localhost:8085](http://localhost:8085)
- Use `admin` and `uptimelabs` as the username and password.
- Click on the `Add realm` button and select the `config/tenants-realm.json` file to import and click on the `Create` button.

### 5.3 Removing packages

```bash
$ upctl remove <package name>
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
$ upctl config docker
```

if that doesn't resolve the issue, check if you have configured the image pull secrets for deployments.
