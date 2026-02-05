#!/bin/bash
# test shell state persistence across multiple calls

BASE_URL="http://localhost:8084"
API_KEY="test"

# startup
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock"

# set a variable
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" -d "export MY_VAR=hello_world" "$BASE_URL/execute"

# verify variable persists in next call
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d 'echo $MY_VAR' "$BASE_URL/execute")
if [ "$response" != "hello_world" ]; then
  echo "variable should persist: expected 'hello_world', got '$response'"
  exit 1
fi

# change directory and verify it persists
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" -d "cd /tmp" "$BASE_URL/execute"
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "pwd" "$BASE_URL/execute")
if [ "$response" != "/tmp" ]; then
  echo "directory should persist: expected '/tmp', got '$response'"
  exit 1
fi

# define a function and call it in next request
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" -d "greet() { echo \"Hello, \$1\"; }" "$BASE_URL/execute"
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "greet World" "$BASE_URL/execute")
if [ "$response" != "Hello, World" ]; then
  echo "function should persist: expected 'Hello, World', got '$response'"
  exit 1
fi

# set multiple variables and verify all persist
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" -d "export A=1 B=2 C=3" "$BASE_URL/execute"
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d 'echo "$A$B$C"' "$BASE_URL/execute")
if [ "$response" != "123" ]; then
  echo "multiple variables should persist: expected '123', got '$response'"
  exit 1
fi

exit 0
