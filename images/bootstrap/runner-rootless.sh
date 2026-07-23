#!/bin/bash

# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright the KubeVirt Authors.

set -eu -o pipefail

# TODO:
# - /usr/local/bin/create_bazel_cache_rcs.sh
# - "${GOPATH}/bin"
# - "${HOME}/containers/auth.json"

at_exit()
{
    local script

    # Do not exit on error so that all the teardown mixins are executed
    set +e

    for script in \
        /etc/teardown.mixin.d/bootstrap/*.sh \
        $(find /etc/teardown.mixin.d/ -maxdepth 1 -name '*.sh')
    do
        echo "[INFO] Running teardown mixin ${script}" >&2
        source "${script}"
    done

    rm -rf "${XDG_RUNTIME_DIR}"
}

trap at_exit EXIT

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/${UID}}"
if [[ ! -d "${XDG_RUNTIME_DIR}" ]]; then
    XDG_RUNTIME_DIR=$(mktemp -d "${TMPDIR:-/tmp}/$(id -un)-private.XXXXXXXX")
fi

# Run setup mixins
for script in \
    /etc/setup.mixin.d/bootstrap/*.sh \
    $(find /etc/setup.mixin.d/ -maxdepth 1 -name '*.sh')
do
    echo "[INFO] Running setup mixin ${script}" >&2
    source "${script}"
done

# Use a reproducible build date based on the most recent git commit timestamp.
if git rev-parse --git-dir >/dev/null 2>&1; then
    SOURCE_DATE_EPOCH=$(git log -1 --format='%ct')
    export SOURCE_DATE_EPOCH
fi

EXIT_VALUE='0'
(set -x; exec "$@") || EXIT_VALUE=$?

exit "${EXIT_VALUE}"
