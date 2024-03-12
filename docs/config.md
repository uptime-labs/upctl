## 1. Quick Intro to repositories and packages

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

