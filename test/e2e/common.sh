set -euxo pipefail

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

apply_model() {
  model_name=$1
  model_file="$REPO_DIR/manifests/models/$model_name.yaml"
  
  if [ -n "${CACHE_PROFILE:-}" ]; then
    yq eval ".spec.cacheProfile = \"$CACHE_PROFILE\"" "$model_file" | kubectl apply -f -
  else
    kubectl apply -f "$model_file"
  fi
}