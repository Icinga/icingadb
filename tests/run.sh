#!/bin/sh

set -euxo pipefail

build() {
  pushd ../
  GOOS=linux CGO_ENABLED=0 go build -o tests/ ./cmd/icingadb
  popd
  go test -o icingadb-test -c .
}

cleanup() {
  docker rm -f $(docker ps -q --filter="name=icinga-testing-*") > /dev/null 2>&1 || true
  docker network prune -f --filter="label=icinga=testing"
}

raiseLimits() {
  if [ "$(ulimit -n)" -lt "1024" ]; then
      ulimit -n 1024
  fi
}

run() {
  export ICINGA_TESTING_ICINGADB_BINARY=icingadb
  export ICINGA_TESTING_ICINGADB_SCHEMA=../schema/mysql/schema.sql
  if [ "$(uname -s)" = "Darwin" ]; then
    export TMPDIR="/private${TMPDIR}"
  fi

  ./icingadb-test -icingatesting.debuglog debug.log -test.v
}

# Note that we do not trap ERR as it can be useful to check running containers.
trap 'cleanup' INT TERM
trap 'catch $? $LINENO' EXIT
catch() {
  if [ "$1" -eq "0" ]; then
    cleanup
  fi
}

cd "${BASH_SOURCE%/*}"

cleanup
raiseLimits
build
run
