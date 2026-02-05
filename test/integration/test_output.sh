#!/bin/bash
# test command output handling

BASE_URL="http://localhost:8084"
API_KEY="test"

# startup
curl -s -o /dev/null -X POST -H "X-Shell-Key: $API_KEY" "$BASE_URL/lock"

# test multiline output (normalize line endings from PTY)
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo -e 'line1\nline2\nline3'" "$BASE_URL/execute" | tr -d '\r')
expected=$'line1\nline2\nline3'
if [ "$response" != "$expected" ]; then
  echo "multiline output failed: expected '$expected', got '$response'"
  exit 1
fi

# test output with special characters
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo 'hello\$world'" "$BASE_URL/execute" | tr -d '\r')
if [ "$response" != 'hello$world' ]; then
  echo "special char output failed: expected 'hello\$world', got '$response'"
  exit 1
fi

# test numeric output
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo \$((2 + 2))" "$BASE_URL/execute" | tr -d '\r')
if [ "$response" != "4" ]; then
  echo "numeric output failed: expected '4', got '$response'"
  exit 1
fi

# test empty output command
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "true" "$BASE_URL/execute" | tr -d '\r')
if [ "$response" != "" ]; then
  echo "empty output failed: expected '', got '$response'"
  exit 1
fi

# test command with stderr (stderr goes to PTY, should appear in output)
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo 'stdout'; echo 'stderr' >&2" "$BASE_URL/execute" | tr -d '\r')
if [[ "$response" != *"stdout"* ]] || [[ "$response" != *"stderr"* ]]; then
  echo "stderr handling failed: got '$response'"
  exit 1
fi

# test output without trailing newline ( printf, head -c, echo -n )
response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "printf 'no_newline'" "$BASE_URL/execute" | tr -d '\r')
if [ "$response" != "no_newline" ]; then
  echo "printf without newline failed: expected 'no_newline', got '$response'"
  exit 1
fi

response=$(curl -s -X POST -H "X-Shell-Key: $API_KEY" -d "echo -n 'echo_n_test'" "$BASE_URL/execute" | tr -d '\r')
if [ "$response" != "echo_n_test" ]; then
  echo "echo -n failed: expected 'echo_n_test', got '$response'"
  exit 1
fi

exit 0
