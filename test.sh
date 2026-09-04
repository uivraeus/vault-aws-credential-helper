#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${1:-vault-aws-credential-helper:latest}"

echo "Running unit tests (containerized Go toolchain, no real Vault involved)..."
make test

echo "Building ${IMAGE_NAME}..."
# Deliberately built for the host's own platform (no --platform given), so
# the smoke tests below can `docker run` it directly. `make image` (used for
# the actual release) targets linux/amd64 explicitly instead -- see Makefile.
docker build -t "${IMAGE_NAME}" .

echo "Running container smoke tests (config validation only, still no real Vault)..."

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

run_container() {
    # Captures stdout/stderr separately, echoes the exit code.
    local stdout_file="$1" stderr_file="$2"
    shift 2
    set +e
    docker run --rm "${IMAGE_NAME}" "$@" >"$stdout_file" 2>"$stderr_file"
    local code=$?
    set -e
    echo "$code"
}

# 1. Missing required configuration -> exit code 2, empty stdout, clear stderr message
CODE=$(run_container "$WORKDIR/out1" "$WORKDIR/err1" credential_process)
if [ "$CODE" -eq 2 ] && [ ! -s "$WORKDIR/out1" ] && grep -q "missing required configuration" "$WORKDIR/err1"; then
    echo "Test 1 passed: missing configuration reported cleanly (exit code 2, empty stdout)"
else
    echo "Test 1 failed: exit=$CODE stdout=$(cat "$WORKDIR/out1") stderr=$(cat "$WORKDIR/err1")"
    exit 1
fi

# 2. Unknown subcommand -> exit code 2
CODE=$(run_container "$WORKDIR/out2" "$WORKDIR/err2" bogus)
if [ "$CODE" -eq 2 ] && grep -q "unknown subcommand" "$WORKDIR/err2"; then
    echo "Test 2 passed: unknown subcommand rejected (exit code 2)"
else
    echo "Test 2 failed: exit=$CODE stderr=$(cat "$WORKDIR/err2")"
    exit 1
fi

# 3. No subcommand -> exit code 2, usage message
CODE=$(run_container "$WORKDIR/out3" "$WORKDIR/err3")
if [ "$CODE" -eq 2 ] && grep -q "usage:" "$WORKDIR/err3"; then
    echo "Test 3 passed: missing subcommand shows usage (exit code 2)"
else
    echo "Test 3 failed: exit=$CODE stderr=$(cat "$WORKDIR/err3")"
    exit 1
fi

echo "All tests passed successfully!"
