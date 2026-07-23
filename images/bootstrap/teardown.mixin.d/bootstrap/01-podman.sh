#!/hint/bash

podman::teardown()
{
    if [[ ! -f "${PODMAN_SERVICE_PID_FILE-}" ]]; then
        return
    fi

    local podman_service_pid
    podman_service_pid=$(cat "${PODMAN_SERVICE_PID_FILE}")

    podman container stop --all --time='60'
    podman container rm --all --force

    if [[ -n "${podman_service_pid}" ]]; then
        pkill -P "${podman_service_pid}"
        pidwait -P "${podman_service_pid}"
    fi

    rm -f "${PODMAN_SERVICE_PID_FILE}"
}

if [[ "${PODMAN_IN_CONTAINER_ENABLED-}" == true ]]; then
    echo "[INFO] Stop Podman service..." >&2
    podman::teardown
fi

unset -f podman_service::teardown
