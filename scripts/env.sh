#!/usr/bin/env bash
# Usage: source scripts/env.sh <environment>
# Environments: local, pre-prod

case "${1:-}" in
  local)
    export HTTP_PROXY=http://localhost:9079
    export SNYK_API=http://localhost:8085
    export SNYK_CFG_ORG=d67ab91e-2549-4d6f-b3c4-033bb98e8e98
    export SNYK_TENANT_ID=9900b2b0-cea4-472a-a33b-2478c74552d5
    echo "Environment set to local"
    ;;
  pre-prod)
    unset HTTP_PROXY
    export SNYK_API=https://api.dev.snyk.io
    unset SNYK_CFG_ORG
    unset SNYK_TENANT_ID
    echo "Environment set to pre-prod"
    ;;
  *)
    echo "Usage: source scripts/env.sh <local|pre-prod>"
    ;;
esac
