#!/hint/bash

gcloud::teardown()
{
    # Logout
    if [[ -f "${GOOGLE_APPLICATION_CREDENTIALS-}" ]]; then
        gcloud auth revoke
    fi
}

echo "[INFO] Google Cloud SDK..." >&2
gcloud::teardown

unset -f gcloud::teardown
