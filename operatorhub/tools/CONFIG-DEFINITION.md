# config.yaml file for Bundle Generation

**Note:** For a concrete examples refer `config.yaml` in specific platform directory in tektoncd/operator/operatorhub.

## Quick Intro

```bash
platform: "openshift"
operator-packagename: "openshift-pipelines-operator-rh"


# specify images to be subsituted in the CSV file in a gerated bundle
image-substitutions:

# for replacing container image in a deployment listed in a CSV file
- image: <full name of new image>
  replaceLocations:
    containerTargets:
    - deploymentName: <name of deployment where this image should be substituted>
      containerName: <name of the container within the deployment>

# eg:
- image: registry.redhat.io/openshift-pipelines/pipelines-rhel8-operator@
  replaceLocations:
    containerTargets:
    - deploymentName: openshift-pipelines-operator
      containerName: openshift-pipelines-operator


# for replacing other images (in command in ENV values ...)
- image: <full name of new image>
  replaceLocations:
    envTargets:
    - deploymentName: <name of deployment where this image should be added/updated ENV value>
      containerName: <name of the container within the deployment where this image should be added/updated in a ENV value>
      envKeys:
      - <ENV key name>

#eg
- image: registry.redhat.io/openshift-pipelines/pipelines-operator-proxy-rhel8@
  replaceLocations:
    envTargets:
    - deploymentName: openshift-pipelines-operator
      containerName: openshift-pipelines-operator
      envKeys:
      - IMAGE_PIPELINES_PROXY

# list aditional images which are not used in image replacement, but which should be
supported for disconnected install
# add thrid party images which are not replaced by operator
# but pulled directly by tasks here
defaultRelatedImages: []
#- image: "" ##<imagename>:<tag> or <imagename>@<sha>
#  name: "" # ENV key name value

```
