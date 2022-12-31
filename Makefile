SHELL = /bin/bash

export RED=\033[0;31m
export NC=\033[0m # No Color
export PROJECT_NAMESPACE=uptimelabs
export CLUSTER_NAME=riddler
export KIND_CONFIG_FILE_NAME=config/kind.config.yaml
export OSL=$(shell uname -s | tr '[:upper:]' '[:lower:]')
#export KIND_EXPERIMENTAL_DOCKER_NETWORK=bm-kind
export MYSQL_PASSWORD=$(shell kubectl get secret --namespace mysql mysql -o jsonpath="{.data.mysql-root-password}" --context kind-${CLUSTER_NAME} 2>/dev/null | base64 -d )
export MYSQL_HOST=$(shell kubectl get svc -n mysql mysql -o=jsonpath='{.status.loadBalancer.ingress[0].ip}' --context kind-${CLUSTER_NAME} 2>/dev/null)

export ARCH="arm64"
ifeq ("x86_64", $(uname -m))
	export ARCH="amd64"
endif

.DEFAULT_GOAL:=help

init: create display configure ## Initialize a kind cluster and install core components.

##@ Cluster operations
create: ## Creates a kind cluster.
# check for existing cluster
ifneq (,$(findstring $(CLUSTER_NAME),$(shell kind get clusters 2>/dev/null)))
	@echo -e "Node(s) already exist for a cluster with the name \"riddler\"\n"
else
	kind create cluster --name ${CLUSTER_NAME} --config=${KIND_CONFIG_FILE_NAME}
endif

display: ## Display cluster information.
	@kubectl cluster-info --context kind-${CLUSTER_NAME}

delete: ## Deletes the kind cluster, use docker volume purge to remove any volumes if required.
	@echo -e "${RED}Are you sure you want to delete cluster, ${CLUSTER_NAME}?${NC}" 
	@read -n 1 -r; \
	if [[ $$REPLY =~ ^[Yy] ]]; \
	then \
		kind delete cluster --name ${CLUSTER_NAME}; \
	fi

configure: ## Install core cluster components, metallb, secret manager etc.
	@echo -e "Installing MetalLB load balancer...⏳\n"
	@kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.7/config/manifests/metallb-native.yaml --context kind-${CLUSTER_NAME}
	@echo -e "\nWaiting for the deployment ready...⏳"
	@kubectl wait pods -n metallb-system -l app=metallb --for condition=Ready --timeout=90s
	@echo -e "\n"
	@make -s network-conf
    # install sealed secrets
# helm install sealed-secrets -n kube-system --set-string fullnameOverride=sealed-secrets-controller sealed-secrets/sealed-secrets  --kube-context kind-${CLUSTER_NAME}

services: ## List all the services and IPs.
	@kubectl get svc -A -o yaml  --context kind-${CLUSTER_NAME} | yq -r '.items[] | select(.spec.type=="LoadBalancer") | .metadata.name + " -> " + .status.loadBalancer.ingress[0].ip + ":" + .spec.ports.[].port'

##@ Package (helm) operations
conf-helm: ## Setup help repositories.
	@echo -e "Configuring repositories...\n"
	@for k in `yq eval '. | keys | .[]' repositories.yaml`; do \
		name=`yq eval ".[$$k].name" repositories.yaml`; \
		url=`yq eval ".[$$k].url" repositories.yaml`; \
		command="helm repo add $${name} $${url}"; \
		echo -e "Configuring $${name}....⏳"; \
		$${command}; \
	done
	@echo -e "\n"; \

install-pkgs: conf-helm ## Install and configure development dependencies defined in package.yaml.
	@for k in `yq eval '. | keys | .[]' package.yaml`; do \
		name=`yq eval ".[$$k].name" package.yaml`; \
		repo=`yq eval ".[$$k].repo" package.yaml`; \
		namespace=`yq eval ".[$$k].namespace" package.yaml`; \
		override=`yq eval ".[$$k].override" package.yaml`; \
		command="helm upgrade"; \
		if [ "$${override}" != "null" ]; then \
			export command="$${command} -f overrides/$${override}"; \
		fi; \
		command="$${command} --install $${name} $${repo} -n $${namespace} --wait --create-namespace --kube-context kind-$${CLUSTER_NAME}"; \
		echo -e "Deploying package $${namespace}/$${name}....⏳\n"; \
		$${command}; \
		echo -e "\n"; \
	done

remove-pkgs: ## Remove all installed packages.
	@for k in `yq eval '. | keys | .[]' package.yaml`; do \
		name=`yq eval ".[$$k].name" package.yaml`; \
		repo=`yq eval ".[$$k].repo" package.yaml`; \
		namespace=`yq eval ".[$$k].namespace" package.yaml`; \
		command="helm upgrade"; \
		command="helm uninstall $${name} -n $${namespace} --kube-context kind-$${CLUSTER_NAME}"; \
		echo -e "Removing package $${namespace}/$${name}....\n"; \
		$${command}; \
		echo -e "\n"; \
	done

##@ System dependencies and tools
install-tools: ## Install kind and yq tools on the local machine.
	curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/v0.17.0/kind-${OSL}-amd64
	chmod +x ./kind
	sudo mv ./kind /usr/local/bin/kind

	curl -Lo ./yq https://github.com/mikefarah/yq/releases/latest/download/yq_${OSL}_amd64
	chmod +x ./yq
	sudo mv ./yq /usr/local/bin/yq

##@ Data operations
import-db: ## Import mysql data from the S3.
ifeq (,$(wildcard uptimelabs.sql))
	@echo -e "Logging into to AWS account...⏳\n"
	@tsh app login awsconsole-prod --aws-role TeleportReadOnly
	@echo -e "Coping SQL backup from AWS S3...⏳"
	@tsh aws s3 cp s3://428265895497-prod-wordpress-backups/uptimelabs/uptimelabs.sql .
else
	$(warn uptimelabs.sql exist, skipping download!)
endif
	@echo -e "Importing data to local mysql cluster...⏳"
	@mysql -u root -p${MYSQL_PASSWORD} -h ${MYSQL_HOST} uptimelabs < uptimelabs.sql

##@ Network operations
network-conf: ## Configuring the MetalLB network.
	@echo -e "Configuring MetalLB network...⏳"
	@kubectl apply -f config/metallb-config.yaml -n metallb-system --context kind-${CLUSTER_NAME}

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
