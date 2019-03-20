#!/bin/bash

set -e
set -o pipefail

function has-changes {
    test "$(git diff --stat |wc -l)" -gt 0
}

git config user.email "you@example.com"
git config user.name "Your Name"

if has-changes; then
    HAD_CHANGES=true

    # Just to be sure
    git stash
else
    HAD_CHANGES=false
fi

go fmt

if has-changes; then
    echo
    git diff --stat
    echo
    echo 'Please comply to the code style guide by running "go fmt" locally.'

    false
fi

if $HAD_CHANGES; then
    git stash pop
fi
