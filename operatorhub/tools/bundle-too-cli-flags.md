# Bundle Generation Tool ClI Flags

## List of supported flags

```bash
usage: bundle.py [-h] --workspace <dir path> (--fetch-strategy-local | --fetch-strategy-release-manifest) [--release-manifest <release-manifest file path>]
                 (--upgrade-strategy-semver | --upgrade-strategy-replaces) --operator-release-version OPERATOR_RELEASE_VERSION
                 [--operator-release-previous-version OPERATOR_RELEASE_PREVIOUS_VERSION] --channels CHANNELS --default-channel DEFAULT_CHANNEL
                 [--addn-annotations <key1>=<val1>,<key2><val2>,...<keyn>=<valn>] [--addn-labels <key1>=<val1>,<key2><val2>,...<keyn>=<valn>] [--verbose]

OperatorHub Artifacts Tooling

optional arguments:
  -h, --help            show this help message and exit
  --workspace <dir path>
                        Path to bundle generation workspace dir, this dir should contain config.yaml for image replacements, manifests/ and manifests/kustomization.yaml
                        file for globbing resource manifests. Release artifacts will be writte to <workspace>/release-artifacts
  --fetch-strategy-local
                        aggregate Operator Resources local kustomize flow
  --fetch-strategy-release-manifest
                        aggregate Operator Resources from releasemanifest and example CRs from local kustomize flow
  --release-manifest <release-manifest file path>
                        path to release manifest file, while using 'release-manifest' strategy
  --upgrade-strategy-semver
                        OperatorHub upgrades operator based on operator semver version
  --upgrade-strategy-replaces
                        OperatorHub upgrades operator based on 'spec.replaces: <previous-version>'
  --operator-release-version OPERATOR_RELEASE_VERSION
                        version
  --operator-release-previous-version OPERATOR_RELEASE_PREVIOUS_VERSION
                        previous version
  --channels CHANNELS   channels
  --default-channel DEFAULT_CHANNEL
                        default channel
  --olm-skip-range OLM_SKIP_RANGE
                        value for olm.skipRange annotation in CSV file in the bundle
  --addn-annotations <key1>=<val1>,<key2><val2>,...<keyn>=<valn>
                        additional annotations to be added to CSV file
  --addn-labels <key1>=<val1>,<key2><val2>,...<keyn>=<valn>
                        additional labels to be added to CSV file
  --verbose             run in verbose mode
```