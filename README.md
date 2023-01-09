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
- make
- Helm 3.2.0+
- yq
- kubectl
- mysql-client
- awscli
- WSL2 - Windows Only

### Optional dependencies
- kind v0.17.0 or later - We recommend using `Kind` if you are on GNU-Linux.

## Setting up local-stack

### 1.  Creating a cluster

#### 1.1 Kind cluster - if you have docker desktop, skip to step 1.2

```bash
$ make init
```

This command will create a local Kubernetes cluster using Docker containers as the nodes. You can then use kubectl, the Kubernetes command-line interface, to deploy and manage applications on the cluster.

#### 1.2 Enable Kubernetes cluster in Docker Desktop

- Open up the settings screen and Navigate to the Kubernetes tab, then check Enable Kubernetes:

<img src="./docs/docker.png" width="500"/>


### 2. Installing Packages and Services

We use helm as a package manager to install packages into the Kubernetes clusters. To simlplyfy the management of repositories and the packages the local-stack includes a set of scripts and configuration files.

- **repositories.yaml**
  
  This fie contains a list of required helm repositories to pull helm packages. When you are configuring repositories you can obtain the repository URL from the maintainer usualy from (https://artifacthub.io/) and give any prefered name for the name feild.
  Following is an exmaple of configuring the superset repository, you can give any unique `name` to the name property.

  ```yaml
  - name: superset
    url: https://apache.github.io/superset
  ```

- **package.yaml**

  This file contains all the packages that required to be installed into the local Kubernetes cluster. To define a installable package there are several properties.

  - `name` - The name of helm package. This will be used as a installation name and the name will be prepended to the Kubernetes resources created by this helm package. You can give any prefered name for this.
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

To simply install the pre-defined packages excute this command in the local-stack directory.

```bash
$ make install-packages
```