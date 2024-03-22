#!/bin/sh

# A GitHub issue with this title and those labels (might be just one)
# will be created or, if it already exists and is open, will be reused.
GH_ISSUE_TITLE="Go Module Major Version Updates"
GH_ISSUE_LABELS="dependencies"

set -eu

# UPDATE_MSG will be altered from within check_updates()
UPDATE_MSG=""

# check_updates DIR check if any major updates are within DIR.
# Found updates are being added to UPDATE_MSG
check_updates() {
  available_updates="$(gomajor list -major -dir "$1" 2>&1 \
    | grep -v "no module versions found" \
    | awk '{ print NR ". `" $0 "`" }')"

  if [ -z "$available_updates" ]; then
    echo "Nothing to do in $1"
    return
  fi

  echo "Found $(echo "$available_updates" | wc -l) updates in $1"
  UPDATE_MSG="$(cat <<EOF
$UPDATE_MSG

### Updates in \`$1\`
$available_updates

EOF
  )"
}

for DIR in "$@"; do
  check_updates "$DIR"
done

if [ -z "$UPDATE_MSG" ]; then
  echo "Nothing to do at all :-)"
  exit 0
fi

UPDATE_MSG="$(cat <<EOF
## $GH_ISSUE_TITLE
There are major version updates available for used Go modules.

$UPDATE_MSG
EOF
  )"

active_issue="$(gh issue list \
  --label "$GH_ISSUE_LABELS" \
  --state "open" \
  --search "in:title \"$GH_ISSUE_TITLE\"" \
  --json "number" \
  --jq ".[0].number")"

if [ -n "$active_issue" ]; then
  gh issue comment "$active_issue" \
    --body "$UPDATE_MSG"
else
  gh issue create \
    --title "$GH_ISSUE_TITLE" \
    --label "$GH_ISSUE_LABELS" \
    --body "$UPDATE_MSG"
fi
