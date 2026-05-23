#!/usr/bin/env bash

# This script eases running the integration tests on the local machine.
#
# Configuration happens mostly through environment variables. In addition to
# the already defined variables, the following are introduced for this script.
#
# - ICINGADB_TESTING_RUN_PREFIX
#   Optional command to prefix the actual test run with, e.g., sudo.
# - ICINGADB_TESTING_DATABASES
#   Space-separated list of database types, defaults to "mysql pgsql".
#
# In addition to the environment variables, all arguments given to this script
# are being passed to the go test call, e.g., -test.run to limit the tests.
#
# Example usage:
# $ ICINGADB_TESTING_DATABASES=pgsql ./tests/run.sh -test.run TestHistory

set -euxo pipefail

: "${ICINGADB_TESTING_DATABASES:=mysql pgsql}"
: "${ICINGADB_TESTING_RUN_PREFIX:=}"

build() {
  export GOOS=linux
  export CGO_ENABLED=0

  pushd ..
  go build -o tests/ ./cmd/icingadb
  popd

  go test -o icingadb-test -c .

  pushd sql
  go test -o ../icingadb-test-sql -c .
  popd
}

cleanup() {
  for id in $(docker ps -q --filter="name=icinga-testing-*"); do
    docker container rm -f -v "$id"
  done
  docker network prune -f --filter="label=icinga=testing"
}

raiseLimits() {
  if [ "$(ulimit -n)" -lt "1024" ]; then
    ulimit -n 1024
  fi
}

run() {
  # NOTE: Bind-mounting within the runner container into a further
  # worker container requires a path on the host system. That's why
  # ICINGA_TESTING_ICINGADB_BINARY has an absolute path on the host and a temporary
  # TMPDIR is mounted at the exact same location.

  export ICINGA_TESTING_ICINGADB_BINARY="${PWD}/icingadb"
  export ICINGA_TESTING_ICINGADB_SCHEMA_MYSQL="/app/schema/mysql/schema.sql"
  export ICINGA_TESTING_ICINGADB_SCHEMA_PGSQL="/app/schema/pgsql/schema.sql"

  for database_type in $ICINGADB_TESTING_DATABASES; do
    export ICINGADB_TESTS_DATABASE_TYPE="$database_type"

    for test_run in icingadb-test icingadb-test-sql; do
      TMPDIR="$(mktemp -d)"
      export TMPDIR

      time \
        $ICINGADB_TESTING_RUN_PREFIX \
        docker run \
          --name="icinga-testing-runner-$(dd if=/dev/random count=1 bs=8 2>/dev/null | xxd -p)" \
          --rm \
          --tty \
          --pull=always \
          --network=host \
          --env "TMPDIR=${TMPDIR}" \
          --env-file <(env | grep '^ICINGA') \
          --volume "${PWD}/..":/app \
          --volume /var/run/docker.sock:/var/run/docker.sock \
          --volume "${TMPDIR}:${TMPDIR}" \
          --workdir /app \
          alpine:3 \
          "./tests/${test_run}" \
            -icingatesting.debuglog "tests/debug-${test_run}-${ICINGADB_TESTS_DATABASE_TYPE}.log" -test.v "$@" \
        | tee "debug-${test_run}-${ICINGADB_TESTS_DATABASE_TYPE}-runner.log"
    done
  done
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
run "$@"
