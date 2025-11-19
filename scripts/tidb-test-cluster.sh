#!/usr/bin/env bash

# This file creates minimal TiDB cluster for tests

REALPATH=$(which realpath)
if [ -z $REALPATH ]; then
  realpath() {
    [[ $1 == /* ]] && echo "$1" || echo "$PWD/${1#./}"
  }
fi

# Set up constants
SCRIPT_PATH=$(realpath $(dirname "$0"))
DOCKER=$(which docker)
SCRIPT_INIT=false
DOCKER_NETWORK="mysql_provider_test_network"
RUNNING_CONTAINERS=""
export MYSQL_PORT=${MYSQL_PORT:-4000}
export TAG_VERSION="v${MYSQL_VERSION:-7.5.1}"

# Sanity checks
if [ -z "$DOCKER" ]; then
  echo "Missing docker binary"
  exit 2
fi

# A better class of script...
set -o errexit  # Exit on most errors (see the manual)
set -o errtrace # Make sure any error trap is inherited
set -o nounset  # Disallow expansion of unset variables
set -o pipefail # Use last non-zero exit code in a pipeline

function script_usage() {
  cat <<EOF  | column -t -s"^"
Usage:
  --init ^ Init TiDB cluster
  --destroy ^ Destroy resources
  --port <MYSQL_PORT> ^ TiDB Listen port (default: 4000)
  --version <MYSQL_VERSION> ^ TiDB version (default: 7.5.1)
  -h|--help ^ Displays this help
EOF
}

function parse_params() {
  local param
  while [[ $# -gt 0 ]]; do
    param="$1"
    case $param in
    --port)
			shift
			if [ -z "$1" ]; then
				echo "Missing port"
				exit 1
			fi
      export MYSQL_PORT=${1:-4000}
			shift
      ;;
    --version)
			shift
			if [ -z "$1" ]; then
				echo "Missing version"
				exit 1
			fi
      export TAG_VERSION="v${1:-6.1.0}"
			shift
      ;;

    --init)
      export SCRIPT_INIT=true
			shift
      ;;
    --destroy)
			if [ -z "$SCRIPT_INIT" ]; then
				echo "Can't destroy and init at once"
				exit 1
			fi
			destroy_cluster
			exit 0
      ;;
    -h | --help)
      script_usage
			shift
      exit 0
      ;;
    *)
      echo "Invalid parameter was provided: $param"
			script_usage
			shift
      exit 1
      ;;
    esac
  done
}

function destroy_cluster() {
	RUNNING_CONTAINERS=$(${DOCKER} ps -a -q -f name=tidb -f name=tikv -f name=pd)
	echo "==> Removing any existing TiDB cluster components"
	if [ ! -z "$RUNNING_CONTAINERS" ]; then
		${DOCKER} rm -f $RUNNING_CONTAINERS >/dev/null 2>&1
	fi
	${DOCKER} network rm ${DOCKER_NETWORK} >/dev/null 2>&1 || true
}

function show_docker_logs_and_exit() {
	CONTAINERS=$(docker ps -a -q -f name=$1)
	if [ ! -z "$CONTAINERS" ]; then
		echo "=== Container logs for $1 ==="
		echo "$CONTAINERS" | xargs docker logs --details 2>&1 || true
	else
		echo "No container found for $1 - container may have failed to start"
		echo "Checking if Docker image exists..."
		docker images pingcap/$1:$TAG_VERSION || echo "Image pingcap/$1:$TAG_VERSION not found locally"
	fi
	echo ""
	echo "Error with $1 component. For debugging use:"
	echo "docker ps -a -q -f name=$1 |xargs docker logs"
	exit 1
}

function run_pd() {
	echo "==> Pulling up PD component"
	if ! ${DOCKER} run -d --name pd \
		-v /etc/localtime:/etc/localtime:ro \
		-h pd \
		--network "$DOCKER_NETWORK" \
		pingcap/pd:$TAG_VERSION \
		--name="pd" \
		--data-dir="/data" \
		--client-urls="http://0.0.0.0:2379" \
		--advertise-client-urls="http://pd:2379" \
		--peer-urls="http://0.0.0.0:2380" \
		--advertise-peer-urls="http://pd:2380" \
		--initial-cluster="pd=http://pd:2380" 2>&1; then
		echo "ERROR: Failed to start PD container"
		show_docker_logs_and_exit pd
	fi
}

function run_tikv() {
	echo "==> Pulling up TiKV component"
	if ! ${DOCKER} run -d --name tikv \
		-v /etc/localtime:/etc/localtime:ro \
		-h tikv \
		--network "$DOCKER_NETWORK" \
		pingcap/tikv:$TAG_VERSION \
		--addr="0.0.0.0:20160" \
		--advertise-addr="tikv:20160" \
		--status-addr="0.0.0.0:20180" \
		--data-dir="/data" \
		--pd="pd:2379" 2>&1; then
		echo "ERROR: Failed to start TiKV container"
		show_docker_logs_and_exit tikv
	fi
}

function run_tidb() {
	local _mysql_port=$1
	echo "==> Pulling up TiDB component"
	if ! ${DOCKER} run -d --name tidb \
		-p $_mysql_port:$_mysql_port \
		-v /etc/localtime:/etc/localtime:ro \
		-h tidb \
		--network "$DOCKER_NETWORK" \
		pingcap/tidb:$TAG_VERSION \
		--store=tikv \
		-P $_mysql_port \
		--path="pd:2379" 2>&1; then
		echo "ERROR: Failed to start TiDB container"
		show_docker_logs_and_exit tidb
	fi
}

function main() {
	  parse_params "$@"
		if [[ "$SCRIPT_INIT" = "true" ]]; then
			echo "==> Pulling up TiDB cluster with TiKV and TB components"
			destroy_cluster && \
			${DOCKER} network create ${DOCKER_NETWORK} && \
			run_pd && \
			run_tikv && \
			run_tidb $MYSQL_PORT

		else
			script_usage
		fi
}

main "$@"
