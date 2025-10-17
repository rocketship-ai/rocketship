#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

pushd "${REPO_ROOT}" >/dev/null
ROCKETSHIP_LOG=ERROR rocketship run -af examples/http-openapi-form/rocketship.yaml

set +e
OUTPUT=$(ROCKETSHIP_LOG=ERROR rocketship run -af error-examples/http-openapi-form-invalid/rocketship.yaml 2>&1)
set -e
popd >/dev/null

if ! echo "$OUTPUT" | grep -q "openapi request validation failed"; then
  echo "❌ Expected OpenAPI validation failure for invalid form payload"
  echo "$OUTPUT"
  exit 1
fi

echo "✅ HTTP OpenAPI form/json integration passed"
