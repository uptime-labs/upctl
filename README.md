# The UptimeLabs Kubernetes Local Development Stack

local-stack is a collection of tools that help develop and test UptimeLabs on a local machine.

The stack includes:
- Kubernetes cluster (kind)
- MySQL server 8
- Keycloak
- NodeRED
- MQTT
- Wordpress
- Superset
- and application components

## TL;DR

```bash
$ make
```

## Before you begin

### Prerequisites

Kubernetes in Docker (kind) is a tool for running local Kubernetes clusters using Docker container "nodes".
Install Docker on your local machine. You will need Docker in order to use kind.

- make
- Helm 3.2.0+
- kind v0.17.0
- yq

## Creating a cluster

```bash
$ make init
```

This will create a local Kubernetes cluster using Docker containers as the nodes. You can then use kubectl, the Kubernetes command-line interface, to deploy and manage applications on the cluster.