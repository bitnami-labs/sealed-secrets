# GKE

## Install

If installing on a GKE cluster you don't have admin rights for, a ClusterRoleBinding may be needed to successfully deploy the controller in the final command.  Replace <your-email> with a valid email, and then deploy the cluster role binding:

```bash
USER_EMAIL=<your-email>
kubectl create clusterrolebinding $USER-cluster-admin-binding --clusterrole=cluster-admin --user=$USER_EMAIL
```

# Private GKE clusters

If you are using a **private GKE cluster**, `kubeseal` won't be able to fetch the public key from the controller
because there is firewall that prevents the master to talk directly to the nodes.

There are currently three workarounds:

## Port forward option

You can set the local port option which defines an open port on your local system. When defined, `kubeseal` will port forward the controller to the specified local port on your system. This gets around the firewall issue as it utilizes the `kube-proxy` services on the node instead of trying to communicate directly with the controller over http.

```bash
kubeseal --local-port=43234 <secret.yaml

// Or you can define the environment variable somewhere permanently.
export SEALED_SECRETS_LOCAL_PORT=43234
kubeseal <secret.yaml
```

## Offline sealing

If you have the public key for your controller, you can seal secrets without talking to the controller.
Normally `kubeseal --fetch-cert` can be used to obtain the certificate for later use, but in this case the firewall prevents us from doing it.

The controller outputs the certificate to the logs so you can copy paste it from there.

Once you have the cert this is how you seal secrets:

```bash
kubeseal --cert=cert.pem <secret.yaml
```

## Master-to-Node firewall

You are required to create a Master-to-Node firewall rule to allow GKE to communicate to the kubeseal container endpoint port tcp/8080.

```bash
CLUSTER_NAME=foo-cluster
gcloud config set compute/zone your-zone-or-region
```

Get the `MASTER_IPV4_CIDR`.

```bash
MASTER_IPV4_CIDR=$(gcloud container clusters describe $CLUSTER_NAME \
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
echo $MASTER_IPV4_CIDR $NETWORK $NETWORK_TARGET_TAG

# example output
10.0.0.0/28 foo-network gke-foo-cluster-c1ecba83-node
```

Create the firewall rule.

```bash
gcloud compute firewall-rules create gke-to-kubeseal-8080 \
  --network "$NETWORK" \
  --allow "tcp:8080" \
  --source-ranges "$MASTER_IPV4_CIDR" \
  --target-tags "$NETWORK_TARGET_TAG" \
  --priority 1000
```
