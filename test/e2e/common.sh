set -e

output() {
  ( set +x; echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $*" )
}

retry() {
  local retries=$1
  shift
  local count=0

  until "$@"; do
    exit_code=$?
    count=$((count + 1))
    if [ "$count" -lt "$retries" ]; then
      output "Attempt $count/$retries failed with exit code $exit_code. Retrying..."
      sleep 1  # Optional delay between retries
    else
      output "Command failed after $count attempts."
      return $exit_code
    fi
  done || true  # Prevent 'set -e' from exiting on failed command
}