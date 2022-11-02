#!/bin/bash

set -eo pipefail

find_license_file() {
  MOD_NAME="$1"
  LICENSE_DIR="vendor/$MOD_NAME"
  LICENSE_FILES=({,../}{,UN}LICENSE{,.txt,.md})

  for LICENSE_FILE in "${LICENSE_FILES[@]}"; do
    LICENSE_FILE="${LICENSE_DIR}/$LICENSE_FILE"

    if [ -e "$LICENSE_FILE" ]; then
      echo "$LICENSE_FILE"
      return
    fi
  done

  echo "Module ${MOD_NAME}: license file missing in ${LICENSE_DIR}. Tried:" "${LICENSE_FILES[@]}" >&2
  false
}

list_all_deps() {
  for MAIN_MOD in ./cmd/*; do
    go list -deps "$MAIN_MOD"
  done
}

COMPATIBLE_LINE=$(($LINENO + 2))

COMPATIBLE=(
  # public domain
  3cee2c43614ad4572d9d594c81b9348cf45ed5ac # vendor/github.com/vbauerster/mpb/v6/UNLICENSE
  # MIT
  66d504eb2f162b9cbf11b07506eeed90c6edabe1 # vendor/github.com/cespare/xxhash/v2/LICENSE.txt
  1513ff663e946fdcadb630bed670d253b8b22e1e # vendor/github.com/davecgh/go-spew/spew/../LICENSE
  90a1030e6314df9a898e5bfbdb4c6176d0a1f81c # vendor/github.com/jmoiron/sqlx/LICENSE
  # BSD-2
  8762249b76928cb6995b98a95a9396c5aaf104f3 # vendor/github.com/go-redis/redis/v8/LICENSE
  d550c89174b585d03dc67203952b38372b4ce254 # vendor/github.com/pkg/errors/LICENSE
  # BSD-3
  b23b967bba92ea3c5ccde9962027cd70400865eb # vendor/github.com/google/uuid/LICENSE
  604b38b184689a3db06a0617216d52a95aea10d8 # vendor/github.com/pmezard/go-difflib/difflib/../LICENSE
  # MPLv2
  0a2b84dd9b124c4d95dd24418c3e84fd870cc0ac # vendor/github.com/go-sql-driver/mysql/LICENSE
)

MY_DIR="$(dirname "$0")"

go mod vendor

for MOD_NAME in $(list_all_deps | "${MY_DIR}/ls-deps.pl"); do
  LICENSE_FILE="$(find_license_file "$MOD_NAME")"

  "${MY_DIR}/anonymize-license.pl" "$LICENSE_FILE"
  tr -d ., <"$LICENSE_FILE" | tr \\n\\t ' ' | sponge "$LICENSE_FILE"
  perl -p0 -i -e 's/  +/ /g; s/ +$//; $_ = lc' "$LICENSE_FILE"

  for SHA1 in "${COMPATIBLE[@]}"; do
    if sha1sum -c <<<"$SHA1  $LICENSE_FILE" >/dev/null 2>&1; then
      continue 2
    fi
  done

  echo "Module ${MOD_NAME}: unknown license. Run 'go mod vendor' (or see below), verify by yourself whether" \
    "$LICENSE_FILE is GPLv2 compatible and (if yes) update the license text hashes list at ${0}:$COMPATIBLE_LINE" \
    "and eventually .github/workflows/compliance/anonymize-license.pl:7" >&2

  sha1sum "$LICENSE_FILE"
  head "$LICENSE_FILE"
  false
done
