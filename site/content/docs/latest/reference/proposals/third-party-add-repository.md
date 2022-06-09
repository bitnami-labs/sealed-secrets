# Add a repository and trigger a chart deployment from an external source

## Objective

Enable an external user-interface for chart repositories, such as Harbor, to trigger the addition of a repository in Kubeapps or trigger the deployment of a chart directly into the Kubeapps UI.

## Motivation

Kubeapps focuses presenting a catalog of apps from multiple repositories and enabling easy deployment of those apps. Kubeapps does not (and should not) focus on the management of package repositories nor populating them with apps.

Other projects, such as Harbor, focus specifically on the management of repositories for cloud-native resources such as images and apps. These project don't necessarily focus on the user experience for deploying these apps or images.

Enabling these projects to integrate with Kubeapps enables users to move seamlessly from uploading a new app to a repository through to testing the deployment of that same app in their configured Kubeapps installation. Or from preparing a repository with specific apps to enabling users in Kubeapps to access those apps.

## Goals and non goals

- Enable 3rd party sites to provide an "Add repository to Kubeapps" action which results in the Kubeapps installation scanning and including the repository in its catalog.
- As a consequence, the 3rd party can then link to directly deploy a chart from a repository in the Kubeapps UI.

## Assumptions

- The same OpenID Connect identity provider must be used for both the 3rd party (such as Harbor) and Kubeapps (and by implication, the Kubernetes cluster).
- RBAC policies in the cluster enable users to deploy apps in specific namespaces to which they have access.
- RBAC policies in the cluster enable admins to create AppRepository custom resources which populate the Kubeapps catalog.

### A note regarding RBAC and Kubeapps user access to charts

A single Kubeapps installation does allow users with different RBAC permissions and it is their assigned Kubernetes RBAC which determines what they can do in Kubeapps. This can be used to limit users to specific kubernetes resources such as namespaces (ie. so a user can only access a single namespace) or package repositories (ie. so only an admin can create or view package repositories). But importantly in this current context, it **does not** currently limit access to available charts within the Kubeapps application catalogue once the index of a repository has been imported. A single Kubeapps installation has _one_ catalog which all users of that installation can access. Just like the helm cli which Kubeapps emulates, this catalog is the union of all apps for the configured repositories.

This is a concern from the Harbor dev's: they would like different repositories within a single Kubeapps installation to be available only to those users who have access to the specific project repository in Harbor.

Currently this is not possible with Kubeapps because:

- the catalog provided by Kubeapps is not per namespace, but rather is the union of all configured package repositories, just like the helm CLI it emulates, and
- Kubeapps does not maintain a mapping of user privileges or internal RBAC - it relies on the Kubernetes cluster RBAC alone.

As we move toward Helm 3 by default, we could potentially resolve this by ensuring that AppRepositories, and the charts they reference, are per-namespace, so that the charts available can be limited to the repos in the namespaces to which the user has access. This would then mean that access to a project in Harbor would be equivalent to access to the Kubernetes namespace and could be controlled easily with oauth2 groups or otherwise configured, but this is outside the scope of this document.

Given the current constraints, if an external application, such as harbor, enables a chart repository for a Kubeapps installation the charts of that repository will form part of the single app catalog available to all users of that installation. Although it is less than ideal, external applications could target different Kubeapps installations for different repos, but until Helm 3 is default and we have a chance to limit package repositories to namespaces, this may be the best option given the constraints. The important thing is that users of any integration understand that by enabling the repository in Kubeapps, it will currently mean that all Kubeapps users have access to the charts of the repository.

## User stories

- As a repository maintainer in Harbor, I want to add my new repository to Kubeapps, so that Kubeapps users can deploy charts from my repository.
- As a contributor to a Harbor repository, I want to test the installation of a chart after uploading to Harbor, so that I can verify the user installation experience.
- As a repository maintainer in Harbor, I want to remove my repository from Kubeapps, so that users can no longer install apps from my repository until further notice.

## Implementation

### Kubeapps

Kubeapps provides an api endpoint for package repositories:

- Create Package Repository:

  - URL: /api/backend/v1/apprepositories
  - Method: POST
  - Data

  ```grpc
  syntax = "proto3";
  message CreateAppRepositoryRequest {
    message AppRepository {
      string name = 1;
      string url = 2;
    }
    // Later add credentials

    AppRepository app_repository = 1;
  }
  ```

  or JSON equivalent

  ```json
  {
    "appRepository": {
      "name": "foo",
      "url": "https://example.com/stable"
    }
  }
  ```

  - Success response: 201 Created with the following data:

  ```grpc
  message CreateAppRepositoryResponse {
    string repository_prefix = 1; // e.g. "/#/charts/<repository-name>/"
  }
  ```

  or JSON equivalent

  ```json
  {
    "repositoryPrefix": "/#/charts/my-repo/"
  }
  ```

  - Error responses
    - 401 Unauthorized
    - 400 Bad Request
    - 409 Conflict
    - Body of error responses to be defined.

- Delete Package Repository
  - URL: /api/backend/v1/apprepositories/<app-repo-name>
  - Method: DELETE
  - Success response: 200 OK
  - Error responses
    - 401 Unauthorized
    - Body of error responses to be defined.

### Third Parties

Third party apps can then:

- POST to the above to create a new repository in Kubeapps, using their OpenID Connect `id_token` as the bearer, or alternatively, (if the request is being sent from the browser) first authenticating with Kubeapps in a separate tab/iframe to set the oauth2_proxy cookie. Assuming the associated identity has RBAC permissions to create repositories in the cluster, the call will succeed.
- Provide links to deploy a chart using the `repositoryPrefix`, eg. `/#/charts/my-repo/<chart-name>`

Though it is not necessary, third-party apps can be configured with multiple Kubeapps installations if required for different users and/or clusters.

## Questions

- Why not enable deploying the chart without adding the repository?
  - At this point in time, Kubeapps allows charts to be installed from repositories only, relying on other services to parse and provide all chart metadata (READMEs, values.yaml, etc.) for use by Kubeapps.
- ...
