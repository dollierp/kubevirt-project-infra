#!/hint/bash

gcloud::setup()
{
    # Add gcloud utilities to $PATH
    if [[ -f /google-cloud-sdk/path.bash.inc ]]; then
        source /google-cloud-sdk/path.bash.inc
    fi

    gcloud config set core/disable_usage_reporting true
    gcloud config set component_manager/disable_update_check true

    # Login
    if [[ -f "${GOOGLE_APPLICATION_CREDENTIALS-}" ]]; then
        gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"
    fi
}

echo "[INFO] Google Cloud SDK..." >&2
gcloud::setup

unset -f gcloud::setup
