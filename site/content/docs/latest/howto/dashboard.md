# Using the Dashboard

Once you have [installed Kubeapps in your cluster](https://github.com/vmware-tanzu/kubeapps/tree/main/chart/kubeapps) you can use the Dashboard to start managing and deploying applications in your cluster. Checkout the [Getting Started](../tutorials/getting-started.md) guide to learn how to access the Dashboard and deploy your first application.

The following sections walk you through some common tasks with the Kubeapps Dashboard.

## Work with Charts

### Deploy new applications using the Dashboard

- Start with the Dashboard welcome page:

  ![Dashboard main page](../img/dashboard-home.png)

- Use the "Catalog" menu to select an application from the list of applications available. This example assumes you want to deploy MariaDB.

  ![MariaDB chart](../img/mariadb-chart.png)

- Click the "Deploy" button. You will be prompted for the release name, cluster namespace and values for your application deployment.

  ![MariaDB installation](../img/mariadb-installation.png)

- Click the "Submit" button. The application will be deployed. You will be able to track the new Kubernetes deployment directly from the browser. The "Notes" section of the deployment page contains important information to help you use the application.

  ![MariaDB deployment](../img/mariadb-deployment.png)

### List all the applications running in your cluster

The "Applications" page displays a list of the application deployments in your cluster.

![Deployment list](../img/dashboard-deployments.png)

### Remove existing application deployments

You can remove any of the applications from your cluster by clicking the "Delete" button on the application's status page:

![Deployment removal](../img/dashboard-delete-deployment.png)

### Add more chart repositories

By default, Kubeapps comes with the Bitnami repository enabled. You can see the list of enabled chart repositories in the "Package Repositories" page under the menu:

![Repositories List](../img/dashboard-repos.png)

Add new repositories (for example, your organization's chart repository) by clicking the "Add Package Repository" button. Fill the "Add Repository" form using the repository info. For a detailed guide of how to add package repositories, check [this guide](./private-app-repository.md).

![Adding repository](../img/dashboard-add-repo.png)
