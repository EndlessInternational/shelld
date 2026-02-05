#!/bin/bash
# test basic flow: startup, run command, status, shutdown

BASE_URL="http://localhost:8084"
API_KEY="test"

# test startup
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock")
if [ "$status" != "200" ]; then
  echo "startup failed: expected 200, got $status"
  exit 1
fi

# test status
response=$(curl -s -H "X-Shell-Key: $API_KEY" "$BASE_URL/state")
if [ "$response" != "locked" ]; then
  echo "status check failed: expected 'locked', got '$response'"
  exit 1
fi

# test run command
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo hello" "$BASE_URL/execute")
if [ "$response" != "hello" ]; then
  echo "run command failed: expected 'hello', got '$response'"
  exit 1
fi

# test health endpoint ( no auth )
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
if [ "$status" != "200" ]; then
  echo "health check failed: expected 200, got $status"
  exit 1
fi

exit 0
