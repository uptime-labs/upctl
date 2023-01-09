SHELL = /bin/bash

export RED=\033[0;31m
export NC=\033[0m # No Color
export PROJECT_NAMESPACE=uptimelabs
export CONTEXT_NAME=docker-desktop
export KIND_CONFIG_FILE_NAME=config/kind.config.yaml
export ECR_REPO='300954903401.dkr.ecr.eu-west-1.amazonaws.com'
export OSL=$(shell uname -s | tr '[:upper:]' '[:lower:]')
#export KIND_EXPERIMENTAL_DOCKER_NETWORK=bm-kind
export MYSQL_PASSWORD=$(shell kubectl get secret --namespace mysql mysql -o jsonpath="{.data.mysql-root-password}" --context ${CONTEXT_NAME} 2>/dev/null | base64 -d )

# mysql hostname. we are using ip here because mysql try to connect to local socket
export MYSQL_HOST=127.0.0.1
ifeq ("docker-desktop",${CONTEXT_NAME})
	export MYSQL_HOST=$(shell kubectl get svc -n mysql mysql -o=jsonpath='{.status.loadBalancer.ingress[0].ip}' --context ${CONTEXT_NAME} 2>/dev/null)
endif

export ARCH="arm64"
ifeq ("x86_64", $(uname -m))
	export ARCH="amd64"
endif

.DEFAULT_GOAL:=help

##@ Kind Cluster operations
init: create display configure ## Initialize a kind cluster and install core components.

create: ## Creates a kind cluster.
# check for existing cluster
ifneq (,$(findstring $(CONTEXT_NAME),$(shell kind get clusters 2>/dev/null)))
	@echo -e "Node(s) already exist for a cluster with the name \"riddler\"\n"
else
	kind create cluster --name ${CONTEXT_NAME} --config=${KIND_CONFIG_FILE_NAME}
endif

display: ## Display cluster information.
	@kubectl cluster-info --context kind-${CONECONTEXT_NAMEXT_NAME}

delete: ## Deletes the kind cluster, use docker volume purge to remove any volumes if required.
	@echo -e "${RED}Are you sure you want to delete cluster, ${CONTEXT_NAME}?${NC}" 
	@read -n 1 -r; \
	if [[ $$REPLY =~ ^[Yy] ]]; \
	then \
		kind delete cluster --name ${CONTEXT_NAME}; \
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

network-conf: ## Configuring the MetalLB network.
	@echo -e "Configuring MetalLB network...⏳"
	@kubectl apply -f config/metallb-config.yaml -n metallb-system --context ${CONTEXT_NAME}

##@ Kubernetes Cluster operations
services: ## List all the services and IPs.
ifneq ("docker-desktop",${CONTEXT_NAME})
	@kubectl get svc -A -o yaml  --context ${CONTEXT_NAME} | yq -r '.items[] | select(.spec.type=="LoadBalancer") | .metadata.name + " -> " + .status.loadBalancer.ingress[0].hostname + ":" + .spec.ports.[].port'
else
	@kubectl get svc -A -o yaml  --context ${CONTEXT_NAME} | yq -r '.items[] | select(.spec.type=="LoadBalancer") | .metadata.name + " -> " + .status.loadBalancer.ingress[0].ip + ":" + .spec.ports.[].port'
endif

uptimelabs-cfg: ## creates uptimelabs namespace, secrets and configmaps for services
	-@kubectl create ns uptimelabs --context ${CONTEXT_NAME}
	@kubectl apply -f config/uptimelabs-env-secret.yaml -n uptimelabs --context ${CONTEXT_NAME}
	@kubectl apply -f config/uptimelabs-env-configmap.yaml -n uptimelabs --context ${CONTEXT_NAME}

create-image-pullsecret: uptimelabs-cfg ## create a pull secret in uptimelabs namespace for ECR private pulls
	@echo -e "Logging into to AWS account...⏳\n"
	@tsh app login awsconsole-shared --aws-role TeleportReadOnly
	$(eval $@_REGPWD := $(shell tsh aws --app awsconsole-shared ecr get-login-password --region eu-west-1))
	@kubectl create secret docker-registry regcred --docker-server=${ECR_REPO} --docker-username=AWS --docker-password=$($@_REGPWD) -n uptimelabs --context ${CONTEXT_NAME}

##@ Package (helm) operations
helm-cfg: ## Setup help repositories.
	@echo -e "Configuring repositories...\n"
	@for k in `yq eval '. | keys | .[]' repositories.yaml`; do \
		name=`yq eval ".[$$k].name" repositories.yaml`; \
		url=`yq eval ".[$$k].url" repositories.yaml`; \
		command="helm repo add $${name} $${url}"; \
		echo -e "Configuring $${name}....⏳"; \
		$${command}; \
	done
	@echo -e "\n"; \

install-pkgs: helm-cfg ## Install and configure development dependencies defined in package.yaml.
	@for k in `yq eval '. | keys | .[]' package.yaml`; do \
		name=`yq eval ".[$$k].name" package.yaml`; \
		repo=`yq eval ".[$$k].repo" package.yaml`; \
		namespace=`yq eval ".[$$k].namespace" package.yaml`; \
		override=`yq eval ".[$$k].override" package.yaml`; \
		command="helm upgrade"; \
		if [ "$${override}" != "null" ]; then \
			export command="$${command} -f overrides/$${override}"; \
		fi; \
		command="$${command} --install $${name} $${repo} -n $${namespace} --wait --create-namespace --kube-context $${CONTEXT_NAME}"; \
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
		command="helm uninstall $${name} -n $${namespace} --kube-context $${CONEXT_NAME}"; \
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
	@tsh aws --app awsconsole-prod s3 cp s3://428265895497-prod-wordpress-backups/uptimelabs/uptimelabs.sql .
else
	$(warn uptimelabs.sql exist, skipping download!)
endif
	@echo -e "Importing data to local mysql cluster...⏳"
	@mysql -u root -p${MYSQL_PASSWORD} -h ${MYSQL_HOST} -P3307 uptimelabs < uptimelabs.sql

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
