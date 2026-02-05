#!/bin/bash
# test shutdown endpoint

BASE_URL="http://localhost:8084"
API_KEY="test"

# startup
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock"

# run a command to verify shell is working
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo test" "$BASE_URL/execute")
if [ "$response" != "test" ]; then
  echo "shell should be working before shutdown: got '$response'"
  exit 1
fi

# call shutdown endpoint
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/unlock")
if [ "$status" != "200" ]; then
  echo "shutdown should return 200: got $status"
  exit 1
fi

# wait for server to shut down
sleep 1

# server should no longer be reachable
status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 "$BASE_URL/health" 2>/dev/null)
if [ "$status" == "200" ]; then
  echo "server should not be reachable after shutdown"
  exit 1
fi

exit 0
