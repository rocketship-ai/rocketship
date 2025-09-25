#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[cli-errors] %s\n' "$1"
}

log "Validating error scenarios"
! rocketship validate nonexistent.yaml
! rocketship run -f nonexistent.yaml
! rocketship validate examples/simple-http/rocketship.yaml examples/nonexistent.yaml

log "Testing run persistence error flows"
! rocketship list -e localhost:7700
! rocketship get abc123 -e localhost:7700

rocketship start server --background
sleep 3
! rocketship get invalid-run-id -e localhost:7700
rocketship stop server
log "CLI error handling checks complete"
