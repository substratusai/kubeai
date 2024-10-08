set -e

retry() {
  local retries=$1
  shift
  local count=0

  set +x
  until "$@"; do
    exit_code=$?
    count=$((count + 1))
    if [ "$count" -lt "$retries" ]; then
      echo -n "."
      sleep 1  # Optional delay between retries
    else
      echo "Command failed after $count attempts."
      set -x
      return $exit_code
    fi
  done || true  # Prevent 'set -e' from exiting on failed command
  echo ""
  set -x
}