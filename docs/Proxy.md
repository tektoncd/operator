<!--
---
linkTitle: "Proxy"
weight: 10
---
-->
# Proxy Support

We have added the support for running the tekton components in a proxy environment through operator.
This will enable running the tekton controllers, webhook and also taskrun pods with proxy configuration.
We are attaching the proxy env variables like HTTP_PROXY, NO_PROXY and HTTPS_PROXY to the pods created
through automation on the fly.

**Note**:
1. This funcationality will only be available for pods created using taskruns, not for all pods on clusters.

### Proxy Support on Kubernetes

For enabling the proxy support on Kubernetes, users need to update the operator deployment with proxy environment 
variables like

```yaml
spec:
  containers:
    - env:
        - name: HTTP_PROXY
          value: "url-to-be-pasted-here"
```

This will restart the operator pod, ending up in proxy environment variables attached to all components pods. Also whenever you run
a taskrun, the resulting pod will also be having the environment variables in all containers.

This functionality of adding proxy environment variables is not available on taskruns created in `tekton-pipelines` namespace.

### Proxy Support on OpenShift

For enabling proxy support on OpenShift environment, configure the proxy environments on OpenShift like 
mentioned [here](https://docs.openshift.com/container-platform/4.7/networking/enable-cluster-wide-proxy.html). 
After that install the operator through Operator Hub, and proxy environment variables will be available in tekton
component's controllers and webhook pod, also all taskrun pods created will also have the these environments

By default, `openshift-pipelines` namespace is exempted from adding the proxy environment varaibles in taskrun pods

#### Configuring namespace to be exempted from proxy environment feature

By default, the automation of adding proxy environment variables to taskrun pod is available on
all namespace except the namespace in which tekton components are installed.

If you want to disable this feature in any other namespace, you can update the namespace
object with following label

```yaml
operator.tekton.dev/disable-proxy: true
```

#### Support for certificates for HTTPS proxy

TBD