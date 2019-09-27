#!/bin/bash

# This script sets up your a local development environment for debugging purposes,
# it requires oc and a valid kube config.

# make sure oc is installed
if ! [ -x "$(command -v oc)" ]; then
  echo "oc command is required but was not found"
  exit 1
fi

# symlink the current kube config to $DEV_DIR_NAME - it will be set in config.json later
if [[ -z "${KUBECONFIG}" ]]; then
  echo "KUBECONFIG env var is required for this script to work, please set it and run the script again"
  exit 1
fi

# create a directory where all files will be stored
DEV_DIR_NAME="dev_env"
mkdir -p $DEV_DIR_NAME

# symlink the current kube config to $DEV_DIR_NAME - it will be set in config.json later
ln -nsf $KUBECONFIG $DEV_DIR_NAME/kubeconf

# copy the current configuration file and append
#   kube config section
#   an empty servingInfo section to prevent openshift-controller-manager from exposing health end point
oc get cm -n=openshift-controller-manager config -o json | jq '.data' | jq '."config.yaml"' | jq 'fromjson' | jq '. += {"kubeClientConfig":{"kubeConfig":"kubeconf"}}' | jq '. += {"servingInfo": {}}' > $DEV_DIR_NAME/config.json
