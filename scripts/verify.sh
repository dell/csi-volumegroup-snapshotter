#!/bin/bash
#
# Copyright (c) 2021 - 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
PROG="${0}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
DARK_GRAY='\033[1;30m'
NC='\033[0m' # No Color

function decho() {
  if [ -n "${DEBUGLOG}" ]; then
    echo "$@" | tee -a "${DEBUGLOG}"
  fi
}

function debuglog_only() {
  if [ -n "${DEBUGLOG}" ]; then
    echo "$@" >> "${DEBUGLOG}"
  fi
}

function log() {
  case $1 in
  separator)
    decho "------------------------------------------------------"
    ;;
  error)
    decho
    log separator
    printf "${RED}Error: $2\n"
    printf "${RED}Installation cannot continue${NC}\n"
    debuglog_only "Error: $2"
    debuglog_only "Installation cannot continue"
    exit 1
    ;;
  step)
    printf "|\n|- %-65s" "$2"
    debuglog_only "${2}"
    ;;
  small_step)
    printf "%-61s" "$2"
    debuglog_only "${2}"
    ;;
  section)
    log separator
    printf "> %s\n" "$2"
    debuglog_only "${2}"
    log separator
    ;;
  smart_step)
    if [[ $3 == "small" ]]; then
      log small_step "$2"
    else
      log step "$2"
    fi
    ;;
  arrow)
    printf "  %s\n  %s" "|" "|--> "
    ;;
  step_success)
    printf "${GREEN}Success${NC}\n"
    ;;
  step_failure)
    printf "${RED}Failed${NC}\n"
    ;;
  step_warning)
    printf "${YELLOW}Warning${NC}\n"
    ;;
  info)
    printf "${DARK_GRAY}%s${NC}\n" "$2"
    ;;
  passed)
    printf "${GREEN}Success${NC}\n"
    ;;
  warnings)
    printf "${YELLOW}Warnings:${NC}\n"
    ;;
  errors)
    printf "${RED}Errors:${NC}\n"
    ;;
  *)
    echo -n "Unknown"
    ;;
  esac
}

function run_command() {
  local RC=0
  if [ -n "${DEBUGLOG}" ]; then
    local ME=$(basename "${0}")
    echo "---------------" >> "${DEBUGLOG}"
    echo "${ME}:${BASH_LINENO[0]} - Running command: $@" >> "${DEBUGLOG}"
    debuglog_only "Results:"
    eval "$@" | tee -a "${DEBUGLOG}"
    RC=${PIPESTATUS[0]}
    echo "---------------" >> "${DEBUGLOG}"
  else
    eval "$@"
    RC=$?
  fi
  return $RC
}

function check_error() {
  if [[ $1 -ne 0 ]]; then
    log step_failure
  else
    log step_success
  fi
}

function print_usage() {
  echo
  echo "Help for $PROG"
  echo
  echo "Usage: $PROG options..."
  echo "The verify script validates V1 snapshots version installed, vgs crd version and vgs-image running in the driver."
  echo "Options:"
  echo "  Optional"
  echo "  -h                                 Help"
  echo

  exit 0
}


while getopts 'h:' flag; do
  case "${flag}" in
    h) print_usage
       exit 0;;
  esac
done


# verify that the no alpha version of volume snapshot resource is present on the system
function verify_alpha_snap_resources() {
  log step "Verifying v1alpha1 snapshot resources"
  decho
  log arrow
  log smart_step "Verifying that v1alpha1 snapshot CRDs are not installed" "small"

  error=0
  # check for the alpha snapshot CRDs. These shouldn't be present for installation to proceed with
  CRDS=("VolumeSnapshotClasses" "VolumeSnapshotContents" "VolumeSnapshots")
  for C in "${CRDS[@]}"; do
    # Verify that alpha snapshot related CRDs/CRs are not there on the system.
    run_command kubectl explain ${C} 2>&1  | grep "^VERSION.*v1alpha1$" --quiet
    if [ $? -eq 0 ]; then
      error=1
      found_error "Expected v1 CRD but found v1alpha1 CRD is installed. Please uninstall it"
      if [[ $(run_command kubectl get ${C} -A --no-headers 2>/dev/null | wc -l) -ne 0 ]]; then
        found_error " Found CR for v1alpha1 CRD ${C}. Please delete it"
      fi
    fi
  done
  check_error error
}

# verify that the requirements for snapshot support exist
function verify_snap_requirements() {
  log step "Verifying snapshot support"
  decho
  log arrow
  log smart_step "Verifying that snapshot CRDs are available" "small"

  error=0
  # check for the CRDs. These are required for installation
  CRDS=("VolumeSnapshotClasses" "VolumeSnapshotContents" "VolumeSnapshots")
  for C in "${CRDS[@]}"; do
    # Verify if snapshot related CRDs are there on the system. If not install them.
    run_command kubectl explain ${C} 2>&1  | grep "^VERSION.*v1$" --quiet
    if [ $? -ne 0 ]; then
      error=1
      found_error "The CRD for ${C} is not installed. These need to be installed by the Kubernetes administrator"
    fi
  done
  check_error error

  log arrow
  log smart_step "Verifying that the snapshot controller is available" "small"

  error=0
  # check for the snapshot-controller. These are strongly suggested but not required
  run_command kubectl describe pod -A | grep snapshot-controller:v4.* 2>&1 >/dev/null
  if [ $? -ne 0 ]; then
    error=1
    found_warning "The Snapshot Controller does not seem to be deployed. The Snapshot Controller should be provided by the Kubernetes vendor or administrator."
  fi

  check_error error
}

# verify Volumegroup-snapshotter CRD
function verify_vgs_crds() {
  log step "Verifying Volumegroup-snapshot CRD"
  decho
  log arrow
  log smart_step "Verifying that vgs CRDs are available" "small"

  error=0
  # check for the CRDs. These are required for installation
  CRDS=("dellcsivolumegroupsnapshots.volumegroup.storage.dell.com")
  for C in "${CRDS[@]}"; do
    # Verify if snapshot related CRDs are there on the system. If not install them.
    run_command kubectl get crd ${C} --no-headers -o=custom-columns='XX:.spec.versions[0].name' 2>&1 >/dev/null
    if [ $? -ne 0 ]; then
      error=1
      found_error "The CRD for ${C} is not installed. Refer install guide."
    fi
  done
  check_error error
}

# verify Volumegroup-snapshotter Label
function verify_vgs_image() {
  log step "Verifying Volumegroup-snapshot Label"
  decho
  log arrow
  log smart_step "Verifying that vgs controller is installed" "small"

  error=0

  # Verify if vgs image is installed. If not install them.
  run_command kubectl describe pods -n vxflexos | grep vg-snapshotter-enabled=true  2>&1 >/dev/null
  if [ $? -ne 0 ]; then
    error=1
    found_error "The Vgs is not installed. Refer install guide."
  fi
  check_error error
}


# found_error, installation will not continue
function found_error() {
  for N in "$@"; do
    ERRORS+=("${N}")
  done
}

# found_warning, installation can continue
function found_warning() {
  for N in "$@"; do
    WARNINGS+=("${N}")
  done
}


verify_alpha_snap_resources
verify_snap_requirements
verify_vgs_crds
verify_vgs_image
