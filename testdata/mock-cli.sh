#!/bin/bash
# Mock CLI for testing subprocess management
# Usage: mock-cli.sh [options]
#   --echo "text"     : Echo text as JSON to stdout
#   --large-output N  : Generate N KB of output (for deadlock testing)
#   --sleep N         : Sleep for N seconds (for signal testing)
#   --spawn-child     : Spawn a child process (for process tree testing)
#   --exit-code N     : Exit with code N

set -e

EXIT_CODE=0
SPAWN_CHILD=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --echo)
      echo "{\"content\": \"$2\"}"
      shift 2
      ;;
    --large-output)
      KB=$2
      LINES=$((KB * 1024 / 20))  # Approximately 20 bytes per line
      for i in $(seq 1 $LINES); do
        echo "{\"line\": $i}"
      done
      shift 2
      ;;
    --sleep)
      SLEEP_TIME=$2
      # Set up SIGTERM handler for graceful shutdown
      trap 'echo "Received SIGTERM, exiting"; exit 143' TERM
      sleep "$SLEEP_TIME" &
      SLEEP_PID=$!
      wait $SLEEP_PID
      shift 2
      ;;
    --spawn-child)
      SPAWN_CHILD=true
      shift
      ;;
    --exit-code)
      EXIT_CODE=$2
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

# Spawn child process if requested
if [ "$SPAWN_CHILD" = true ]; then
  sleep 300 &
  CHILD_PID=$!
  echo "{\"spawned_child\": $CHILD_PID}" >&2
fi

exit $EXIT_CODE
