#!/bin/bash
# test kill functionality
# kill sends Ctrl+C to interrupt current command but keeps shell locked

BASE_URL="http://localhost:8084"
API_KEY="test"

# startup
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock"

# test kill
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/kill")
if [ "$status" != "200" ]; then
  echo "kill failed: expected 200, got $status"
  exit 1
fi

# state should be locked after kill ( shell stays running )
response=$(curl -s -H "X-Shell-Key: $API_KEY" "$BASE_URL/state")
if [ "$response" != "locked" ]; then
  echo "state should be 'locked' after kill: got '$response'"
  exit 1
fi

# shell should still work after kill
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo still_alive" "$BASE_URL/execute")
if [ "$response" != "still_alive" ]; then
  echo "shell should work after kill: expected 'still_alive', got '$response'"
  exit 1
fi

exit 0
