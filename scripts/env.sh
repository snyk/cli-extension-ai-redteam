#!/usr/bin/env bash
# Usage: source scripts/env.sh <environment> [api_port] [proxy_port]
#
# Environments: local, pre-prod
#
# For local, ports default to 8085 (API) and 9079 (proxy).
# Override when running multiple instances (e.g. via `make serve-new`):
#   source scripts/env.sh local 8186 9180

case "${1:-}" in
  local)
    local_api_port="${2:-8085}"
    local_proxy_port="${3:-9079}"
    export HTTP_PROXY="http://localhost:${local_proxy_port}"
    export SNYK_API="http://localhost:${local_api_port}"
    export SNYK_TOKEN=local-dev
    export SNYK_CFG_ORG=d67ab91e-2549-4d6f-b3c4-033bb98e8e98
    export SNYK_TENANT_ID=9900b2b0-cea4-472a-a33b-2478c74552d5
    echo "Environment set to local (API :${local_api_port}, proxy :${local_proxy_port})"
    ;;
  pre-prod)
    unset HTTP_PROXY
    export SNYK_API=https://api.dev.snyk.io
    unset SNYK_TOKEN
    unset SNYK_CFG_ORG
    unset SNYK_TENANT_ID
    echo "Environment set to pre-prod"
    ;;
  *)
    echo "Usage: source scripts/env.sh <local|pre-prod> [api_port] [proxy_port]"
    ;;
esac
