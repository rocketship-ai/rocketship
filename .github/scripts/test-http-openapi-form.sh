#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

pushd "${REPO_ROOT}" >/dev/null
ROCKETSHIP_LOG=ERROR rocketship run -af examples/http-openapi-form/rocketship.yaml
popd >/dev/null

echo "✅ HTTP OpenAPI form integration passed"
