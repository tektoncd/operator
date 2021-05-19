#!/usr/bin/env python3
import genericpath
import re

import yaml
import argparse
import os
import subprocess

WORKSPACE_DIR=""
STRATEGY_LOCAL="local"
STRATEGY_RELEASE_MANIFEST="release-manifest"

def envIndex(keyName, envVars):
    for i, e in enumerate(envVars):
        if e['name'] == keyName:
            return i
    return -1

def generate_bundle(config):
    divider("Generating Bundle")
    artifact_dir = os.path.join(WORKSPACE_DIR, "release-artifacts")
    if not os.path.exists(artifact_dir):
        os.mkdir(artifact_dir)
    os.chdir(artifact_dir)
    kustomize_resources = (
                            "kustomize build "
                            "--load-restrictor LoadRestrictionsNone "
                            "../manifests/strategy-{strategy_name}".format(strategy_name=config["strategy"])
                        )

    aggregate_resources = kustomize_resources
    if config["strategy"] == STRATEGY_RELEASE_MANIFEST:
        aggregate_resources = "cat {release_manifest_path} ".format(release_manifest_path=config["release-manifest"])
        aggregate_resources += "<(echo ---QQQ 123 ) "
        aggregate_resources += "<({kustomize_cmd})".format(kustomize_cmd=kustomize_resources)

    cmd = '''
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
        version=config["operator-release-version"]
    )

    proc = subprocess.run(cmd, shell=True)
    return proc.returncode


def divider(title=''):
    print(':' * 25)
    if title:
        print(':' * 5, title)
        print('-' * 25)

def updateCSV(config):
    divider("update csv")
    artifact_dir = os.path.join(WORKSPACE_DIR, "release-artifacts")
    os.chdir(artifact_dir)
    csv_dir = os.path.join(os.getcwd(), "bundle/manifests")
    csv_name = config["operator-packagename"] + ".clusterserviceversion.yaml"

    for filename in os.listdir(csv_dir):
        if not re.match(".*clusterserviceversion.yaml$", filename):
            continue
        with open(os.path.join(csv_dir, filename), 'r+') as csv_stream:
            try:
                csv = yaml.safe_load(csv_stream)
                divider()

                # replaces operator images
                imagesubs = config['image-substitutions']
                relatedImages = config['defaultRelatedImages']
                deployments = csv['spec']['install']['spec']['deployments']
                for imagesub in imagesubs:
                    divider(imagesub["image"])
                    newImage = imagesub["image"]
                    for replaceLoc in imagesub["replaceLocations"]:
                        for deployment in deployments:
                            if deployment["name"] == replaceLoc["deploymentName"]:
                                containers = deployment["spec"]["template"]["spec"]["containers"]
                                for container in containers:
                                    if container["name"] == replaceLoc["containerName"]:
                                        # if no 'replaceAsEnvVars' is specified replace container's image
                                        if (not "replaceAsEnvVars" in replaceLoc):
                                            container["image"] = newImage
                                            image = {
                                                'name': container["name"].upper().replace('-', '_'),
                                                'image': newImage
                                            }
                                            relatedImages.append(image.copy())
                                            continue
                                        for key in replaceLoc["replaceAsEnvVars"]:
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
                csv_stream.seek(0)
                csv_stream.truncate()
                yaml.safe_dump(csv, csv_stream, default_flow_style=False)
                divider()

            except yaml.YAMLError as exc:
                print(exc)
                exit(1)

if __name__ == "__main__":
    divider("BundleGen")
    parser = argparse.ArgumentParser(description='OperatorHub Artifacts Tooling')
    # parser.add_argument('--config-file', help='Path to bundlegen-config-example.yaml file')
    parser.add_argument('--workspace', help='Path to bundle generation workspace dir')
    parser.add_argument('--strategy', help='local/release-manifest, specifies how to aggregate Operator Resources', default='local')
    parser.add_argument('--release-manifest-file', help='path to release manifest file, while using \'release-manifest\' strategy', default='release.yaml')
    args = parser.parse_args()
    WORKSPACE_DIR=args.workspace
    if not os.path.isabs(args.workspace):
        WORKSPACE_DIR=os.path.join(os.getcwd(), args.workspace)

    config_file_path = os.path.join(WORKSPACE_DIR, "config.yaml")
    #load config
    with open(config_file_path, 'r') as stream:
        try:
            config = yaml.safe_load(stream)
        except yaml.YAMLError as exc:
            print(exc)
    if args.strategy == STRATEGY_RELEASE_MANIFEST:
        if not os.path.isabs(args.release_manifest_file):
            config["release-manifest"] = os.path.join(os.getcwd(), args.release_manifest_file)
        if not os.path.exists(config["release-manifest"]):
            print("Release manifest file", config["release-manifest"], "doesnot exist")
            exit(1)
    config["strategy"] = args.strategy

    generate_bundle(config)

    divider()
    if config["strategy"] == STRATEGY_LOCAL:
        updateCSV(config)
