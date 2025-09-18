#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

pushd "${REPO_ROOT}" >/dev/null
ROCKETSHIP_LOG=ERROR rocketship run -af examples/http-openapi-form/rocketship.yaml

set +e
OUTPUT=$(ROCKETSHIP_LOG=ERROR rocketship run -af examples/http-openapi-form/rocketship-invalid.yaml 2>&1)
STATUS=$?
set -e
popd >/dev/null

if [[ $STATUS -eq 0 ]]; then
  echo "❌ Expected invalid form run to fail but it succeeded"
  echo "$OUTPUT"
  exit 1
fi

if ! echo "$OUTPUT" | grep -q "openapi request validation failed"; then
  echo "❌ Expected OpenAPI validation failure for invalid form payload"
  echo "$OUTPUT"
  exit 1
fi

echo "✅ HTTP OpenAPI form/json integration passed"
