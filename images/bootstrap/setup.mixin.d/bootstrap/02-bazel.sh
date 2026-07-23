#!/hint/bash

# Give bazel a well defined output_user_root directory independent of the user
# used in the image. This allows mounting an emptyDir at this location, instead
# of writing into the container overlay.
bazel::setup()
{
    local bazel_output_dir='/tmp/cache/bazel' bazelrcs=()

    mkdir -p "${bazel_output_dir}"

    # https://bazel.build/run/bazelrc
    local bazelrc bazelrc_files=("${HOME}/.bazelrc")

    if [[ ${EUID} == 0 ]]; then
        # This file is used to support installation-wide options or options shared between users.
        # Reading of this file can be disabled using the --nomaster_bazelrc option.
        bazelrc_files+=('/etc/bazel.bazelrc')
    fi

    for bazelrc in "${bazelrc_files[@]}"; do
        printf 'startup --output_user_root=%s\n' "${bazel_output_dir}" >>"${bazelrc}"
    done
}

if [[ "${KUBEVIRT_RUN_UNNESTED-}" == true ]]; then
    echo "[INFO] Bazel..." >&2
    bazel::setup
fi

unset -f bazel::setup
