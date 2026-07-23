#!/bin/bash

set -eu -o pipefail

if [[ ${EUID} == 0 ]]; then
    exec /usr/local/bin/runner-rootful.sh "$@"
else
    exec /usr/local/bin/runner-rootless.sh "$@"
fi
