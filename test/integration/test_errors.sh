#!/bin/bash
# test error handling

BASE_URL="http://localhost:8084"
API_KEY="test"

# test missing key header
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/lock")
if [ "$status" != "401" ]; then
  echo "missing key should return 401: got $status"
  exit 1
fi

# first request with a key locks the shell to that key
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock")
if [ "$status" != "200" ]; then
  echo "first request should lock shell and return 200: got $status"
  exit 1
fi

# different key should now return 401 on protected endpoints
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: wrong-key" -d "echo test" "$BASE_URL/execute")
if [ "$status" != "401" ]; then
  echo "wrong key should return 401 after shell is locked: got $status"
  exit 1
fi

# state requires a key
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/state")
if [ "$status" != "401" ]; then
  echo "state without key should return 401: got $status"
  exit 1
fi

# test empty command
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" -d "" "$BASE_URL/execute")
if [ "$status" != "400" ]; then
  echo "empty command should return 400: got $status"
  exit 1
fi

# note: "run before startup" is tested implicitly - a fresh server has state "available"
# and any run attempt would fail. we can't test it here without shutting down the server.

exit 0
