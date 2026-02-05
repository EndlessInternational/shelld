#!/bin/bash
# test state conflict handling

BASE_URL="http://localhost:8084"
API_KEY="test"

# startup
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock"

# double lock should return 409
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock")
if [ "$status" != "409" ]; then
  echo "double lock should return 409: got $status"
  exit 1
fi

# verify error message
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock")
if [[ "$response" != *"already locked"* ]]; then
  echo "double lock should mention 'already locked': got '$response'"
  exit 1
fi

exit 0
