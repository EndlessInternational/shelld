#!/bin/bash
# test command timeout behavior

BASE_URL="http://localhost:8084"
API_KEY="test"

# startup
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock"

# test timeout with short timeout header
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  -H "X-Shell-Key: $API_KEY" \
  -H "X-Command-Timeout: 500ms" \
  -d "sleep 3" \
  "$BASE_URL/execute")
if [ "$status" != "202" ]; then
  echo "timeout should return 202: got $status"
  exit 1
fi

# status should be executing
response=$(curl -s -H "X-Shell-Key: $API_KEY" "$BASE_URL/state")
if [ "$response" != "executing" ]; then
  echo "status should be 'executing' during command: got '$response'"
  exit 1
fi

# wait for command to complete
sleep 4

# status should be locked
response=$(curl -s -H "X-Shell-Key: $API_KEY" "$BASE_URL/state")
if [ "$response" != "locked" ]; then
  echo "status should be 'locked' after command completes: got '$response'"
  exit 1
fi

# test kill during executing state
curl -s -o /dev/null -X POST \
  -H "X-Shell-Key: $API_KEY" \
  -H "X-Command-Timeout: 500ms" \
  -d "sleep 10" \
  "$BASE_URL/execute"

# should be executing
response=$(curl -s -H "X-Shell-Key: $API_KEY" "$BASE_URL/state")
if [ "$response" != "executing" ]; then
  echo "status should be 'executing': got '$response'"
  exit 1
fi

# kill should work
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/kill")
if [ "$status" != "200" ]; then
  echo "kill should return 200: got $status"
  exit 1
fi

# should be locked after kill ( kill just sends Ctrl+C, keeps shell alive )
sleep 1
response=$(curl -s -H "X-Shell-Key: $API_KEY" "$BASE_URL/state")
if [ "$response" != "locked" ]; then
  echo "status should be 'locked' after kill: got '$response'"
  exit 1
fi

exit 0
