#!/hint/bash

podman::setup()
{
    local {base,podman}_runtime_dir
    base_runtime_dir="${1:?[ERROR] Empty base runtime directory parameter}"; shift
    podman_runtime_dir="${base_runtime_dir}/podman"

    mkdir -m0700 -p "${podman_runtime_dir}"

    local podman_service_log="${podman_runtime_dir}/podman.log"
    PODMAN_SERVICE_PID_FILE="${podman_runtime_dir}/podman.pid"
    PODMAN_SERVICE_SOCKET="${podman_runtime_dir}/podman.sock"

    CONTAINER_HOST="unix://${PODMAN_SERVICE_SOCKET}"

    (
        [[ -n "${CONTAINER_HTTP_PROXY-}" ]] && export HTTP_PROXY="${CONTAINER_HTTP_PROXY}"
        [[ -n "${CONTAINER_HTTPS_PROXY-}" ]] && export HTTPS_PROXY="${CONTAINER_HTTPS_PROXY}"
        [[ -n "${CONTAINER_NO_PROXY-}" ]] && export NO_PROXY="${CONTAINER_NO_PROXY}"

        # https://docs.podman.io/en/latest/markdown/podman-system-service.1.html
        podman system service "${CONTAINER_HOST}" \
            --log-level='info' --time='0' "$@" >"${podman_service_log}" 2>&1 &

        podman_service_pid=$!
        printf '%d\n' "${podman_service_pid}" >"${PODMAN_SERVICE_PID_FILE}"
    )

    # Wait for the Podman service to be ready
    curl -I http://d/_ping \
        --fail-with-body --no-progress-meter \
        --retry 10 --retry-all-errors --retry-max-time 45 \
        --unix-socket "${PODMAN_SERVICE_SOCKET}" \
    || {
        local retcode=$?
        echo "[ERROR] Podman service failed to start."
        cat "${podman_service_log}"
        exit "${retcode}"
    } >&2

    export CONTAINER_HOST
    podman::status >>"${podman_service_log}"

    # Force the Docker client to use the Podman service
    if [[ ${EUID} == 0 ]]; then
        ln -sf /var/run/docker.sock "${PODMAN_SERVICE_SOCKET}"
    else
        export DOCKER_HOST="${CONTAINER_HOST}"
    fi
}

podman::status()
{
    local endpoint

    for endpoint in info _ping; do
        curl -fsS --unix-socket "${PODMAN_SERVICE_SOCKET}" "http://d/${endpoint}"
    done
}

if [[ "${PODMAN_IN_CONTAINER_ENABLED-}" == true ]]; then
    echo "[INFO] Start Podman service..." >&2
    podman::setup "${XDG_RUNTIME_DIR}"
fi

unset -f podman::{setup,status}
