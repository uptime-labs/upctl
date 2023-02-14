# The UptimeLabs Kubernetes Local Development Stack

local-stack is a collection of tools that help develop and test UptimeLabs on a local machine.

The stack includes:
- Kubernetes cluster (kind)
- MySQL server 8
- Keycloak
- NodeRED
- MQTT
- WordPress
- Superset
- and application components

---

- [The UptimeLabs Kubernetes Local Development Stack](#the-uptimelabs-kubernetes-local-development-stack)
  - [Before you begin](#before-you-begin)
    - [Prerequisites](#prerequisites)
    - [Optional dependencies](#optional-dependencies)
  - [Setting up local-stack](#setting-up-local-stack)
    - [1.  Creating a cluster](#1--creating-a-cluster)
      - [1.1 Kind cluster - if you have docker desktop, skip to step 1.2](#11-kind-cluster---if-you-have-docker-desktop-skip-to-step-12)
      - [1.2 Enable Kubernetes cluster in Docker Desktop](#12-enable-kubernetes-cluster-in-docker-desktop)
    - [2. Installing Packages and Services](#2-installing-packages-and-services)


## Before you begin

### Prerequisites

Kubernetes in Docker (kind) is a tool for running local Kubernetes clusters using Docker container "nodes".
Install Docker on your local machine. You will need Docker in order to use kind.

- Docker Desktop
- kubectl
- mysql-client
- awscli
- WSL2 - if you are using Windows

Read more about installing the dependencies [here](docs/dependencies.md)

### Optional dependencies
- kind v0.17.0 or later - We recommend using `Kind` if you are on GNU-Linux.

### 1. Creating a local Kubernetes cluster

#### 1.1 Kind cluster - if you have docker desktop, skip to step 1.2

```bash
$ kind create cluster --cluster config/kind.config.yaml
```

This command will create a local Kubernetes cluster using Docker containers as the nodes. You can then use kubectl, the Kubernetes command-line interface, to deploy and manage applications on the cluster.

#### 1.2 Enable Kubernetes cluster in Docker Desktop

- Open up the settings screen and Navigate to the Kubernetes tab, then check Enable Kubernetes:

<img src="./docs/docker.png" width="500"/>


### 1.3 Configure Helm chart repositories

This command will configure the helm repositories for the local Kubernetes cluster. This is required to pull the helm packages from the helm repositories.

The repositories are configured in the `.upctl.yaml` file. You can add or remove repositories from the file and then run the command to configure the repositories.
for the Uptimelabs private Helm repository you must configure the username and password in the `.upctl.yaml` file.

```bash
$ upctl config r
```

### 1.4 Configure ECR credentials

This command will configure the ECR credentials for the local Kubernetes cluster. This is required to pull the images from the private ECR repository.

```bash
$ upctl config d
```

### 1.5 Install metallb and configure for kind cluster (skip if you are using docker desktop)

- Read more about metallb [here](https://metallb.universe.tf/)
- Read more about network configuration with metallb [here](docs/network.md)

```bash
$ upctl install metallb
$ kubeclt apply -f config/metallb-config.yaml
```

### 2. Installing Packages and Services

We use helm as a package manager to install packages into the Kubernetes clusters. To simplify the management of repositories and the packages the local-stack includes a set of scripts and configuration files.

- **upctl.yaml**

  - `repositories`

    This fie contains a list of required helm repositories to pull helm packages. When you are configuring repositories you can obtain the repository URL from the maintainer usualy from (https://artifacthub.io/) and give any preferred name for the name feild.
    Following is an example of configuring the superset repository, you can give any unique `name` to the name property.

    ```yaml
    - name: superset
      url: https://apache.github.io/superset
    ```

  - `packages`

    This file contains all the packages that required to be installed into the local Kubernetes cluster. To define a installable package there are several properties.

    - `name` - The name of helm package. This will be used as an installation name and the name will be prepended to the Kubernetes resources created by this helm package. You can give any preferred name for this.
    - `repo` - The combination of repo and package names in format `<repo name>/<package name>`. The package name as defined by the helm package maintainer. Repository name is the name given in the `repositories.yaml`.
    - `namespace` - Namespace of the package resources should install into.
    - `override` - Helm value files to override default helm package values.

    Values for each package can be modified using the overrides config files located in the `<root>/overrides` directory.
    
    ```yaml
    - name: mosquitto # package name
      repo: k8shome/mosquitto # hem repository name / chart name
      namespace: uptimelabs # namespace to install the package (this will get automatically created)
      override: mqtt.yaml # helm value file for the configuration override
    ```

- To simply install the pre-defined packages execute this command in the local-stack directory.

```bash
$ upctl install --all
```

To install a specific package

```bash
$ upctl install <package name>
```
