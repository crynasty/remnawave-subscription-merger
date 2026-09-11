#!/usr/bin/env bash

set -euo pipefail

WORKFLOW_FILE="build-dev.yml"
GITHUB_REPOSITORY="crynasty/remnawave-subscription-merger"
GITHUB_API="https://api.github.com"
VERSION="dev"

COMMIT_FULL=$(git rev-parse HEAD 2>/dev/null || echo "")
COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
if [[ -z "${COMMIT_FULL}" ]]; then
  echo "Failed to resolve git commit for current repository" >&2
  exit 1
fi

REF="main"
if [[ -z "${REF}" || "${REF}" == "HEAD" ]]; then
  echo "Failed to resolve git ref. Set GITHUB_REF manually or checkout a branch." >&2
  exit 1
fi

AUTH_TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
if [[ -z "${AUTH_TOKEN}" ]] && command -v gh >/dev/null 2>&1; then
  AUTH_TOKEN="$(gh auth token 2>/dev/null || true)"
fi

if [[ -z "${AUTH_TOKEN}" ]]; then
  echo "No GitHub token found. Use one of: GITHUB_TOKEN, GH_TOKEN, or run 'gh auth login'." >&2
  exit 1
fi

PAYLOAD=$(cat <<EOF
{"ref":"${REF}","inputs":{"version":"${VERSION}","commit":"${COMMIT_FULL}"}}
EOF
)

RESPONSE=$(curl -sS -w $'\n%{http_code}' \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "${GITHUB_API}/repos/${GITHUB_REPOSITORY}/actions/workflows/${WORKFLOW_FILE}/dispatches" \
  -d "${PAYLOAD}")

HTTP_CODE="${RESPONSE##*$'\n'}"
BODY="${RESPONSE%$'\n'*}"

if [[ "${HTTP_CODE}" != "204" ]]; then
  echo "Failed to trigger workflow. HTTP ${HTTP_CODE}" >&2
  if [[ -n "${BODY}" ]]; then
    echo "${BODY}" >&2
  fi
  exit 1
fi

echo "Development build dispatched successfully:"
echo "  repository: ${GITHUB_REPOSITORY}"
echo "  workflow:   ${WORKFLOW_FILE}"
echo "  ref:        ${REF}"
echo "  image:      ghcr.io/crynasty/remnawave-subscription-merger:${VERSION}"
echo "  commit:     ${COMMIT_SHORT}"
