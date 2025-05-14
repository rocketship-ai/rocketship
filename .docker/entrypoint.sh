#!/bin/bash
set -e

# Start the rocketship server in background mode
rocketship start server --local --background

# Wait for the server to be ready (includes waiting for Temporal)
echo "Waiting for rocketship server to be ready..."
sleep 10

# Run the tests based on environment variables
if [ -n "$TEST_FILE" ]; then
    echo "Running single test file: $TEST_FILE"
    exec rocketship run --file "$TEST_FILE" --engine "$ENGINE_HOST"
elif [ -n "$TEST_DIR" ]; then
    echo "Running tests in directory: $TEST_DIR"
    exec rocketship run --dir "$TEST_DIR" --engine "$ENGINE_HOST"
else
    echo "Error: Neither TEST_FILE nor TEST_DIR is set"
    exit 1
fi 