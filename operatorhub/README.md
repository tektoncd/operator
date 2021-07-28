# Generating Bundles (release artifact) for OperatorHub

1. From project root run:

```bash
export BUNDLE_ARGS="--workspace=operatorhub/kubernetes --operator-release-version=1.1.1 --channels=a,b --default-channel=a --fetch-strategy-local --upgrade-strategy-semver"
make bundle
```

