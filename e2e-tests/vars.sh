#!/bin/bash

export ROOT_REPO=${ROOT_REPO:-${PWD}}

export DEPLOY_DIR="${DEPLOY_DIR:-${ROOT_REPO}/deploy}"
export TESTS_DIR="${TESTS_DIR:-${ROOT_REPO}/e2e-tests}"
export TESTS_CONFIG_DIR="${TESTS_CONFIG_DIR:-${TESTS_DIR}/conf}"
export TEMP_DIR=$(mktemp -d)

export GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
export VERSION=${VERSION:-$(echo "${GIT_BRANCH}" | sed -e 's^/^-^g; s^[.]^-^g;' | tr '[:upper:]' '[:lower:]')}

export IMAGE=${IMAGE:-"perconalab/percona-server-mysql-operator:${VERSION}"}
export IMAGE_MYSQL=${IMAGE_MYSQL:-"perconalab/percona-server-mysql-operator:main-ps8.0"}
export IMAGE_ORCHESTRATOR=${IMAGE_ORCHESTRATOR:-"perconalab/percona-server-mysql-operator:main-orchestrator"}
export IMAGE_PMM=${IMAGE_PMM:-"perconalab/pmm-client:dev-latest"}
export PMM_SERVER_VERSION=${PMM_SERVER_VERSION:-$(curl https://raw.githubusercontent.com/Percona-Lab/percona-openshift/main/helm/pmm-server/Chart.yaml | awk '/^version/{print $NF}')}
export IMAGE_PMM_SERVER_REPO="perconalab/pmm-server"
export IMAGE_PMM_SERVER_TAG="dev-latest"

date=$(which gdate || which date)

if oc get projects; then
	export OPENSHIFT=4
fi
