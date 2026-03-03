<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [GKE](#gke)
  - [Install](#install)
  - [Private GKE clusters](#private-gke-clusters)
    - [Offline sealing](#offline-sealing)
    - [Control Plane to Node firewall](#control-plane-to-node-firewall)
- [RBAC and GKE Warden Restrictions](#rbac-and-gke-warden-restrictions)
- [Workarounds](#workarounds)
  - [Option 1: Disable the service-proxier (Simplest)](#option-1-disable-the-service-proxier-simplest)
  - [Option 2: Use Google Groups for RBAC (Recommended)](#option-2-use-google-groups-for-rbac-recommended)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# GKE

## Install

If installing on a GKE cluster you don't have admin rights for, a ClusterRoleBinding may be needed to successfully deploy the controller in the final command.  Replace <your-email> with a valid email, and then deploy the cluster role binding:

```bash
USER_EMAIL=<your-email>
kubectl create clusterrolebinding $USER-cluster-admin-binding --clusterrole=cluster-admin --user=$USER_EMAIL
```

## Private GKE clusters

If you are using a **private GKE cluster**, `kubeseal` won't be able to fetch the public key from the controller
because there is firewall that prevents the control plane to talk directly to the nodes.

There are currently two workarounds:

### Offline sealing

If you have the public key for your controller, you can seal secrets without talking to the controller.
Normally `kubeseal --fetch-cert` can be used to obtain the certificate for later use, but in this case the firewall prevents us from doing it.

The controller outputs the certificate to the logs so you can copy paste it from there.

Once you have the cert this is how you seal secrets:

```bash
kubeseal --cert=cert.pem <secret.yaml
```

### Control Plane to Node firewall

You are required to create a Control Plane to Node firewall rule to allow GKE to communicate to the kubeseal container endpoint port tcp/8080.

```bash
CLUSTER_NAME=foo-cluster
gcloud config set compute/zone your-zone-or-region
```

Get the `CP_IPV4_CIDR`.

```bash
CP_IPV4_CIDR=$(gcloud container clusters describe $CLUSTER_NAME \
  | grep "masterIpv4CidrBlock: " \
  | awk '{print $2}')
```

Get the `NETWORK`.

```bash
NETWORK=$(gcloud container clusters describe $CLUSTER_NAME \
  | grep "^network: " \
  | awk '{print $2}')
```

Get the `NETWORK_TARGET_TAG`.

```bash
NETWORK_TARGET_TAG=$(gcloud compute firewall-rules list \
  --filter network=$NETWORK --format json \
  | jq ".[] | select(.name | contains(\"$CLUSTER_NAME\"))" \
  | jq -r '.targetTags[0]' | head -1)
```

Check the values.

```bash
echo $CP_IPV4_CIDR $NETWORK $NETWORK_TARGET_TAG

# example output
10.0.0.0/28 foo-network gke-foo-cluster-c1ecba83-node
```

Create the firewall rule.

```bash
gcloud compute firewall-rules create gke-to-kubeseal-8080 \
  --network "$NETWORK" \
  --allow "tcp:8080" \
  --source-ranges "$CP_IPV4_CIDR" \
  --target-tags "$NETWORK_TARGET_TAG" \
  --priority 1000
```

Create the firewall rule to see the metrics

```bash
gcloud compute firewall-rules create gke-to-metrics-8081 \
  --network "$NETWORK" \
  --allow "tcp:8081" \
  --source-ranges "$CP_IPV4_CIDR" \
  --target-tags "$NETWORK_TARGET_TAG" \
  --priority 1000
```
# RBAC and GKE Warden Restrictions

On GKE clusters running version `1.32.2-gke.1182003` or later, the **GKE
Warden admission webhook** strictly forbids binding any `Role` or
`ClusterRole` to the `system:authenticated` group.

By default, the `sealed-secrets` Helm chart binds the `service-proxier`
role to this group to allow `kubeseal` to communicate with the
controller and fetch the public key.

On modern GKE versions, this default configuration will cause the
installation to fail with the following error:

``` text
admission webhook "warden-validating.common-webhooks.networking.gke.io" denied the request:
GKE Warden rejected the request because it violates one or more constraints.
Violations details:
{"[denied by rbac-binding-limitation]":["Binding any Role or ClusterRole to Group \"system:authenticated\" is forbidden."]}
```

------------------------------------------------------------------------

# Workarounds

To successfully deploy on GKE, you must override the default
`serviceProxier` settings in your `values.yaml`.

------------------------------------------------------------------------

## Option 1: Disable the service-proxier (Simplest)

If you do not need the `kubeseal --fetch-cert` functionality through the
proxier, you can disable its creation entirely:

``` yaml
rbac:
  serviceProxier:
    create: false
```

------------------------------------------------------------------------

## Option 2: Use Google Groups for RBAC (Recommended)

For a more secure setup, bind the proxier role to a specific restricted
Google Group instead of the broad `system:authenticated` group.

This requires:

1.  Setting up Google Groups for RBAC in your Google Cloud organization.
2.  Creating an "anchor" group named
    `gke-security-groups@yourdomain.com`.
3.  Updating your Helm values to point to your specific subgroup:

``` yaml
rbac:
  serviceProxier:
    subjects:
      - apiGroup: rbac.authorization.k8s.io
        kind: Group
        name: "your-restricted-group@yourdomain.com"
```
