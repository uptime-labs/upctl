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
- kubectl

## Creating a cluster

```bash
$ make init
```

This will create a local Kubernetes cluster using Docker containers as the nodes. You can then use kubectl, the Kubernetes command-line interface, to deploy and manage applications on the cluster.

## Network - Setting up Load Balancer

#### Setup address pool used by loadbalancers

To complete layer2 configuration, we need to provide MetalLB a range of IP addresses it controls. We want this range to be on the docker kind network.

```bash
$ docker network inspect -f '{{.IPAM.Config}}' kind
```

The output will contain a cidr such as 172.18.0.0/16. We want our loadbalancer IP range to come from this subclass. We can configure MetalLB, for instance, to use 172.18.255.200 to 172.19.255.250 by creating the IPAddressPool and the related L2Advertisement.

Update the configuration file `metallb-config.yaml` located in config folder.

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 172.18.255.200-172.18.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
```
