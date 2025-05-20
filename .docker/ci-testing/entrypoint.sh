#!/usr/bin/env bash
set -eo pipefail

# 1. Start everything in the background
rocketship start server --local --background

# 2. Wait until the engine is actually accepting connections
echo "🕐 waiting for Rocketship engine on :7700 ..."
for i in {1..60}; do               # 60‑second timeout
  (echo > /dev/tcp/127.0.0.1/7700) >/dev/null 2>&1 && break
  sleep 1
done || { echo "Timed out waiting for engine"; exit 1; }

# 3. Run the tests
if [[ -n "$TEST_FILE" ]]; then
  echo "▶️  running single test file $TEST_FILE"
  exec rocketship run --file "$TEST_FILE" --engine "${ENGINE_HOST:-127.0.0.1:7700}"
elif [[ -n "$TEST_DIR" ]]; then
  echo "▶️  running tests in dir $TEST_DIR"
  exec rocketship run --dir  "$TEST_DIR" --engine "${ENGINE_HOST:-127.0.0.1:7700}"
else
  echo "Error: neither TEST_FILE nor TEST_DIR is set"
  exit 1
fi
