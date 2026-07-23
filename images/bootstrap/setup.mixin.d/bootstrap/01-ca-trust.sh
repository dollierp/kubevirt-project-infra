#!/hint/bash

ca_trust::setup()
{
    if [[ ${EUID} == 0 ]]; then
        ca_trust::setup::rootful "$@"
    else
        ca_trust::setup::rootless "$@"
    fi
}

ca_trust::setup::rootful()
{
    install -m0644 "${CA_CERT_FILE}" /etc/pki/ca-trust/source/anchors/

    update-ca-trust extract
}

ca_trust::setup::rootless()
{
    local extra_cert_dir="${XDG_RUNTIME_DIR}/pki/certs"
    mkdir -p -m0700 "${extra_cert_dir}"

    install -p -m0644 "${CA_CERT_FILE}" "${extra_cert_dir}"

    openssl rehash -v "${extra_cert_dir}"

    if [[ -z "${SSL_CERT_DIR-}" ]]; then
        SSL_CERT_DIR=$(openssl version -d | awk -F ': ' '{ print($NF) }' | tr -d '"')
    fi
    SSL_CERT_DIR+=":${extra_cert_dir}"

    export SSL_CERT_DIR
}

if [[ -f "${CA_CERT_FILE-}" ]]; then
    echo "[INFO] Adding ${CA_CERT_FILE} as a trusted root CA." >&2
    ca_trust::setup
fi

unset -f ca_trust::setup{,::rootful,rootless}
