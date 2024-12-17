## Setup from scratch

1. [Install Tekton](#install-tekton)
1. [Setup the Tasks and Pipelines](#install-tasks-and-pipelines)
1. [Create the required service account + secrets](#service-account-and-secrets)
1. [Setup post-processing](#setup-post-processing)

### Install Tekton

```bash
# If this is your first time installing Tekton in the cluster you might need to give yourself permission to do so
kubectl create clusterrolebinding cluster-admin-binding-someusername \
  --clusterrole=cluster-admin \
  --user=$(gcloud config get-value core/account)

# Example, Tekton v0.9.1
export TEKTON_VERSION=0.9.1
kubectl apply --filename  https://storage.googleapis.com/tekton-releases/pipeline/previous/v${TEKTON_VERSION}/release.yaml
```

### Install tasks and pipelines

All the `Tasks` required are fetched via remote resolution.
The `Pipeline` can be installed on the cluster:

```shell script
kubectl apply -f tekton/operator-release-pipeline.yaml
```

or it can be executed from the local file using `tkn pipeline start --filename`.

### Service account and secrets

In order to release, these Pipelines use the `release-right-meow` service account,
which uses `release-secret` and has
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
access to google cloud projects:
[`tekton-releases`]((https://github.com/tektoncd/plumbing/blob/master/gcp.md)) and
[`tekton-releases-nightly`]((https://github.com/tektoncd/plumbing/blob/master/gcp.md)).

After creating these service accounts in GCP, the kubernetes service account and
secret were created with:

```bash
KEY_FILE=release.json
GENERIC_SECRET=release-secret
ACCOUNT=release-right-meow

# Connected to the `prow` in the `tekton-releases` GCP project
GCP_ACCOUNT="$ACCOUNT@tekton-releases.iam.gserviceaccount.com"

# 1. Create a private key for the service account
gcloud iam service-accounts keys create $KEY_FILE --iam-account $GCP_ACCOUNT

# 2. Create kubernetes secret, which we will use via a service account and directly mounting
kubectl create secret generic $GENERIC_SECRET --from-file=./$KEY_FILE

# 3. Add the docker secret to the service account
kubectl apply -f tekton/account.yaml
kubectl patch serviceaccount $ACCOUNT \
  -p "{\"secrets\": [{\"name\": \"$GENERIC_SECRET\"}]}"
```

## Supporting scripts and images

Some supporting scripts have been written using Python3:

- [koparse](https://github.com/tektoncd/plumbing/tree/main/tekton/images/koparse) - Contains logic for parsing `release.yaml` files created
  by `ko`
