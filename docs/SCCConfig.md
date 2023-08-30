<!--
---
linkTitle: "SCC Configuration"
weight: 50
---
-->
# Configuring SCC used for Tekton workloads in OpenShift

[Security Context Constraints (SCCs)](https://docs.openshift.com/container-platform/4.13/authentication/managing-security-context-constraints.html)
are used to control permissions for pods in a cluster.

By default, the operator uses a custom SCC called [`pipelines-scc`](https://github.com/tektoncd/operator/blob/e0507dc1f00d5a6a3e8a6d571e595ada0235ae90/cmd/openshift/operator/kodata/openshift/00-prereconcile/openshift-pipelines-scc.yaml)
for all Tekton workloads run on OpenShift. `pipelines-scc` is primarily derived
from the `anyuid` SCC which allows users to run with any UID and GID, and 
hence is not ideal for some, if not most users given the relaxed permissions 
allotted to all workloads. Besides, `pipelines-scc` also allows `SETFCAP` 
Linux capability to be requested by workloads. Historically, these relaxed 
permissions were provided to all Tekton workloads to support use cases like 
building container images via Tekton.

This document describes how to configure the default SCC and other SCC 
related configurations to better suit user's Tekton workloads.

### How to change SCC applied to Tekton workloads

Internally, the operator creates a [service account (SA)](https://kubernetes.io/docs/concepts/security/service-accounts/)
named `pipeline`  and attaches `pipelines-scc` to it by default. The 
`pipeline` SA is attached to all Tekton workloads by the operator and this 
is how the permissions percolate from the attached SCC to the workload pods.

Now the users can attach a different SA to their workloads as well and hence 
`pipelines-scc` will not take effect for those workloads, instead the SCC 
attached to that SA will take effect.

### Configure default and maximum allowed SCC via TektonConfig

**Note: Before we look at how to configure default SCC via the operator, it's 
worth
noting that this feature makes it convenient for users to configure default SCC 
but it does not restrict users from attaching a different SCC via a different SA
to their workloads - and this different SCC could be more  restrictive or 
permissive than the default SCC. So, setting the default SCC should not be 
considered a security control that prohibits users from applying a different 
SCC to their Tekton workloads, instead it's just a way to specify default 
SCC applied by the operator.**

To understand how to configure a default and maximum allowed SCC, let's look 
at an example `TektonConfig` configuration.

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  ...
  ...
  platforms:
    openshift:
      scc:
        default: "restricted-v2"
        maxAllowed: "privileged"
```

- `spec.platforms.openshift.scc.default` specifies the default SCC that
will be attached to the service account used for workloads (`pipeline` SA by
default)
- `spec.platforms.openshift.scc.maxAllowed` specifies the highest SCC that can 
be requested for in any namespace

Note that the SCC specified in `default` field cannot be of a higher priority 
than the one specified in `maxAllowed` field.

#### How are SCCs compared

OpenShift uses a prioritization logic to compare and sort SCCs from most 
restrictive to least restrictive. More on this can be read [here](https://docs.openshift.com/container-platform/4.13/authentication/managing-security-context-constraints.html#scc-prioritization_configuring-internal-oauth). 

### Configuring default SCC for a specific namespace

If users wish to configure a different SCC for Tekton workloads to be run in a 
particular namespace, they can do so by adding the annotation
`operator.tekton.dev/scc: <SCC name>` to their namespace.

Example:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
  annotations:
    operator.tekton.dev/scc: nonroot
```

In this example, in the namespace `test-namespace` all the Tekton workload 
pods will have the effective SCC as `nonroot` irrespective of the default 
SCC set in TektonConfig.

A use case for specifying a different SCC for a given namespace could be
building containers with Tekton. Building containers with, say, buildah in
Tekton needs elevated privileges like running as root and elevated Linux
capabilities. Users can request for `anyuid` SCC in one namespace without
impacting permissions of Tekton workloads running in other namespaces.

**Note: The SCC requested by the `operator.tekton.dev/scc` can not have a 
higher priority than the one specified in `TektonConfig.Spec.Platforms.OpenShift.
SCC.MaxAllowed` field.**
