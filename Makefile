SHELL = /bin/bash

export PROJECT_NAMESPACE=uptimelabs
export CLUSTER_NAME=riddler
export KIND_CONFIG_FILE_NAME=kind.config.yaml
export OSL=$(shell uname -s | tr '[:upper:]' '[:lower:]')

ifeq ("x86_64", $(uname -m))
	export ARCH="amd64"
else
	export ARCH="arm64"
endif

.DEFAULT_GOAL:=help

##@ Init
init: create-cluster display-cluster install-core-components ## initialize a kind cluster and install core components

##@ Create cluster
create-cluster: ## creates a kind cluster
# check for existing cluster
ifneq (,$(findstring $(CLUSTER_NAME),$(shell kind get clusters)))
	@echo -e "Node(s) already exist for a cluster with the name \"riddler\"\n"
else
	kind create cluster --name ${CLUSTER_NAME} --config=${KIND_CONFIG_FILE_NAME}
endif

##@ Display cluster info
display-cluster: ## display cluster information
	kubectl cluster-info --context kind-${CLUSTER_NAME}

##@ Delete cluster
delete-cluster: ## deletes the kind cluster, use docker volume purge to remove any volumes if required
	kind delete cluster --name ${CLUSTER_NAME}

##@ Install dependecies
install-deps: ## install kind and yq dependecies
	curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/v0.17.0/kind-${OSL}-amd64
	chmod +x ./kind
	sudo mv ./kind /usr/local/bin/kind

	curl -Lo ./yq https://github.com/mikefarah/yq/releases/latest/download/yq_${OSL}_amd64
	chmod +x ./yq
	sudo mv ./yq /usr/local/bin/yq

##@ Helm setup
helm-setup: ## setup help repositories
	@echo -e "Configuring repositories...\n"
	@for k in `yq eval '. | keys | .[]' repositories.yaml`; do \
		name=`yq eval ".[$$k].name" repositories.yaml`; \
		url=`yq eval ".[$$k].url" repositories.yaml`; \
		command="helm repo add $${name} $${url}"; \
		echo -e "Configuring $${name}....⏳"; \
		$${command}; \
	done
	@echo -e "\n"; \

##@ Install cluster components
install-core-components: helm-setup ## install core cluster components, nginx ingress, secret manager etc.
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml  --context kind-${CLUSTER_NAME}
	kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller \
  --timeout=100s --context kind-${CLUSTER_NAME}
    # install sealed secrets
# helm install sealed-secrets -n kube-system --set-string fullnameOverride=sealed-secrets-controller sealed-secrets/sealed-secrets  --kube-context kind-${CLUSTER_NAME}

##@ Install packages
install-packages: helm-setup ## Install and configure development dependencies defined in package.yaml
	@for k in `yq eval '. | keys | .[]' package.yaml`; do \
		name=`yq eval ".[$$k].name" package.yaml`; \
		repo=`yq eval ".[$$k].repo" package.yaml`; \
		namespace=`yq eval ".[$$k].namespace" package.yaml`; \
		override=`yq eval ".[$$k].override" package.yaml`; \
		command="helm upgrade"; \
		if [ "$${override}" != "null" ]; then \
			export command="$${command} -f overrides/$${override}"; \
		fi; \
		command="$${command} --install $${name} $${repo} -n $${namespace} --create-namespace --kube-context kind-$${CLUSTER_NAME}"; \
		echo -e "Deploying package $${namespace}/$${name}....⏳\n"; \
		$${command}; \
		echo -e "\n"; \
	done

##@ Remove packages
remove-packages: ## Remove all installed packages
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

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
