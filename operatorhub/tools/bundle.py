#!/usr/bin/env python3
import genericpath
import re
import sys

import yaml
import argparse
import os
import subprocess

WORKSPACE_DIR=""
FETCH_STRATEGY_LOCAL = "fetch-strategy-local"
FETCH_STRATEGY_RELEASE_MANIFEST = "fetch-strategy-release-manifest"
UPGRADE_STRATEGY_SEMVER = "upgrade-strategy-sermver-mode"
UPGRADE_STRATEGY_REPLACE = "upgrade-strategy-replaces-mode"
VERBOSE = False

def buildConfig():
    parser = setParser()
    args = parser.parse_args()
    config, err = baseConfig(args)
    if not err is None:
        parser.error(err)
    config, err = setStrategies(args, config)
    if not err is None:
        parser.error(err)
    config, err = parseLabelsandAnnotations(args, config)
    if not err is None:
        parser.error(err)

    config["release-version"] = args.operator_release_version
    config["channels"] = args.channels
    config["default-channel"] = args.default_channel
    config["verbose"] = args.verbose
    debug(config, yaml.dump(config))
    return config

def setParser():
    parser = argparse.ArgumentParser(description='OperatorHub Artifacts Tooling')
    fetch_strategy = parser.add_mutually_exclusive_group(required=True)
    upgrade_strategy = parser.add_mutually_exclusive_group(required=True)

    parser.add_argument('--workspace',
                        metavar='<dir path>',
                        help='''Path to bundle generation workspace dir,
                                this dir should contain config.yaml for image replacements,
                                manifests/ and manifests/kustomization.yaml file for globbing resource manifests.
                                Release artifacts will be writte to <workspace>/release-artifacts''',
                        required=True)
    fetch_strategy.add_argument('--fetch-strategy-local',
                                help='aggregate Operator Resources local kustomize flow',
                                action='store_const',
                                const=FETCH_STRATEGY_LOCAL)
    fetch_strategy.add_argument('--fetch-strategy-release-manifest',
                                help='aggregate Operator Resources from releasemanifest and example CRs from local kustomize flow',
                                action='store_const',
                                const=FETCH_STRATEGY_RELEASE_MANIFEST)
    parser.add_argument('--release-manifest',
                        metavar='<release-manifest file path>',
                        help='path to release manifest file, while using \'release-manifest\' strategy')
    upgrade_strategy.add_argument('--upgrade-strategy-semver',
                                  help='OperatorHub upgrades operator based on operator semver version',
                                  action='store_const',
                                  const=UPGRADE_STRATEGY_SEMVER)
    upgrade_strategy.add_argument('--upgrade-strategy-replaces',
                                  help='OperatorHub upgrades operator based on \'spec.replaces: <previous-version>\'',
                                  action='store_const',
                                  const=UPGRADE_STRATEGY_REPLACE)

    parser.add_argument('--operator-release-version', help='version', required=True)
    parser.add_argument('--operator-release-previous-version',
                        help='previous version')
    parser.add_argument('--channels', help='channels',required=True)
    parser.add_argument('--default-channel', help='default channel', required=True)
    parser.add_argument('--addn-annotations',
                        help='additional annotations to be added to CSV file',
                        metavar='<key1>=<val1>,<key2><val2>,...<keyn>=<valn>')
    parser.add_argument('--addn-labels',
                        help='additional labels to be added to CSV file',
                        metavar='<key1>=<val1>,<key2><val2>,...<keyn>=<valn>')
    parser.add_argument('--verbose',
                        help='run in verbose mode',
                        action='store_true')
    return parser

def baseConfig(args):
    workspace_dir = args.workspace
    if not os.path.isabs(workspace_dir):
        workspace_dir=os.path.join(os.getcwd(), workspace_dir)
    if not os.path.exists(workspace_dir):
        return None, f"workspace-dir: {args.workspace} doesnot exist"
    config_file_path = os.path.join(workspace_dir, "config.yaml")

    with open(config_file_path, 'r') as stream:
        try:
            config = yaml.safe_load(stream)
        except yaml.YAMLError as exc:
            print(exc)
            exit(1)
    config["workspace"] = workspace_dir
    return config, None

def setStrategies(args, config):
    if args.fetch_strategy_local:
        config["fetch-strategy"] = args.fetch_strategy_local
    elif args.fetch_strategy_release_manifest:
        config["fetch-strategy"] = args.fetch_strategy_release_manifest
        if not args.release_manifest:
            return None, "--releaes-manifest <file path> is required with --fetch-fetch-strategy-release-manifest"
        config["release-manifest"] = args.release_manifest
        if not os.path.isabs(config["release-manifest"]):
            config["release-manifest"] = os.path.join(os.getcwd(), config["release-manifest"])
        if not os.path.exists(config["release-manifest"]):
            return None, f"Release manifest file {config['release-manifest']} doesnot exist"

    if args.upgrade_strategy_replaces:
        config["upgrade-strategy"] = args.upgrade_strategy_replaces
        if not args.operator_release_previous_version:
            return None, "--operator_release_previous_version <n.n.n> is required with --upgrade-strategy-replaces"
        config["previous-release-version"] = args.operator_release_previous_version
    elif args.upgrade_strategy_semver:
        config["upgrade-strategy"] = args.upgrade_strategy_semver
    return config, None

def parseLabelsandAnnotations(args, config):
    if not args.addn_annotations and not args.addn_labels:
        return config, None
    if args.addn_annotations:
        annotations, err = parseKeyVal(args.addn_annotations)
        if err is not None:
            return None, err
        config["addn-annotations"] = annotations
    if args.addn_labels:
        labels, err = parseKeyVal(args.addn_labels)
        if err is not None:
            return None, err
        config["addn-labels"] = labels
    return config, None


def parseKeyVal(keyVals):
    keyValMap = {}
    items = keyVals.rstrip(',').split(',')
    for item in items:
        keyVal = item.split('=')
        if len(keyVal) != 2:
            return None, f"key value pair invalid: {item}"
        keyValMap[keyVal[0]] = keyVal[1]
    return keyValMap, None

def envIndex(keyName, envVars):
    for i, e in enumerate(envVars):
        if e['name'] == keyName:
            return i
    return -1

def generate_bundle(config):
    divider("Generating Bundle")
    cmd = genBundleCmd(config)
    debug(config, "bundle generate command: " + cmd)

    artifact_dir = os.path.join(config["workspace"], "release-artifacts")
    if not os.path.exists(artifact_dir):
        os.mkdir(artifact_dir)
    os.chdir(artifact_dir)
    proc = subprocess.run(cmd, shell=True)
    return proc.returncode

def genBundleCmd(config):
    kustomize_resources = (
        "kustomize build "
        "--load-restrictor LoadRestrictionsNone "
        "../manifests/{strategy_name}".format(strategy_name=config["fetch-strategy"])
    )

    aggregate_resources = kustomize_resources
    if config["fetch-strategy"] == FETCH_STRATEGY_RELEASE_MANIFEST:
        aggregate_resources = "cat {release_manifest_path} ".format(release_manifest_path=config["release-manifest"])
        aggregate_resources += "<(echo --- ) "
        aggregate_resources += "<({kustomize_cmd})".format(kustomize_cmd=kustomize_resources)

    # 'resource-gen | operator-sdk generate bundle'
    # the pipe is required to pass resource list(s) to operator-sdk command because of
    # https://github.com/operator-framework/operator-sdk/issues/4951
    return '''
    {resource_gen} | \
            operator-sdk generate bundle \
                --channels {channels} \
                --default-channel {default_channel} \
                --kustomize-dir manifests \
                --overwrite \
                --package {packagename} \
                --version {version};
    '''.format(
        resource_gen=aggregate_resources,
        channels=config["channels"],
        default_channel=config["default-channel"],
        packagename=config["operator-packagename"],
        version=config["release-version"]
    )

def newCSVmods(config):
    divider("update csv")
    artifact_dir = os.path.join(config["workspace"], "release-artifacts")
    os.chdir(artifact_dir)
    csv_dir = os.path.join(os.getcwd(), "bundle/manifests")
    csv_name = config["operator-packagename"] + ".clusterserviceversion.yaml"

    for filename in os.listdir(csv_dir):
        if not re.match(".*clusterserviceversion.yaml$", filename):
            continue
        with open(os.path.join(csv_dir, filename), 'r+') as csv_stream:
            try:
                csv = yaml.safe_load(csv_stream)

                if config["fetch-strategy"] == FETCH_STRATEGY_LOCAL:
                    csv = imageSub(config, csv)

                csv['spec']['version']
                if config["upgrade-strategy"] == UPGRADE_STRATEGY_REPLACE:
                    csv['spec']['replaces'] = config["previous-release-version"]
                    olm_skipRange = f'>={config["previous-release-version"]} <{config["release-version"]}'
                    csv['metadata']['annotations']['olm.skipRange'] = olm_skipRange
                if "addn-annotations" in config.keys():
                    csv['metadata']['annotations'].update(config["addn-annotations"])
                if "addn-labels" in config.keys():
                    csv['metadata']['labels'].update(config["addn-labels"])

                csv_stream.seek(0)
                csv_stream.truncate()
                yaml.safe_dump(csv, csv_stream, default_flow_style=False)
                divider()

            except yaml.YAMLError as exc:
                print(exc)
                exit(1)

def imageSub(config, csv):
    # replaces operator images
    imagesubs = config['image-substitutions']
    relatedImages = config['defaultRelatedImages']
    csvDeployments = csv['spec']['install']['spec']['deployments']
    yaml.dump(csvDeployments)
    for imagesub in imagesubs:
        newImage = imagesub["image"]
        replaceLocations = imagesub["replaceLocations"]

        if "containerTargets" in replaceLocations.keys():
            for containerTarget in replaceLocations["containerTargets"]:
                for deployment in csvDeployments:
                    if deployment["name"] == containerTarget["deploymentName"]:
                        csvContainers = deployment["spec"]["template"]["spec"]["containers"]
                        for container in csvContainers:
                            if container["name"] == containerTarget["containerName"]:
                                container["image"] = newImage
                                image = {
                                    'name': container["name"].upper().replace('-', '_'),
                                    'image': newImage
                                }
                                relatedImages.append(image.copy())

        if "envTargets" in replaceLocations.keys():
            for envTarget in replaceLocations["envTargets"]:
                for deployment in csvDeployments:
                    if deployment["name"] == envTarget["deploymentName"]:
                        csvDepContainers = deployment["spec"]["template"]["spec"]["containers"]
                        for container in csvDepContainers:
                            if container["name"] == envTarget["containerName"]:
                                for key in envTarget["envKeys"]:
                                    index = envIndex(key, container['env'])
                                    if index != -1:
                                        container['env'][index]['value'] = newImage
                                    else:
                                        envVar = {
                                            'name': key,
                                            'value':newImage
                                        }
                                        container['env'].append(envVar.copy())
                                    image = {
                                        'name': key,
                                        'image': newImage,
                                    }
                                    relatedImages.append(image.copy())

    divider("related images")
    csv['spec']['relatedImages'] = relatedImages
    return csv

def debug(config, message):
    if config["verbose"]:
        print(message)

def divider(title=''):
    print(':' * 25)
    if title:
        print(':' * 5, title)
        print(':' * 25)

if __name__ == "__main__":
    divider("BundleGen")
    config = buildConfig()
    generate_bundle(config)
    newCSVmods(config)
