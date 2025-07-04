#!/usr/bin/env bash
set -euo pipefail

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.2.1
else
    echo "golangci-lint is already installed."
fi

# Check if benchstat is installed
if ! command -v benchstat &> /dev/null; then
    echo "Installing benchstat..."
    go install golang.org/x/perf/cmd/benchstat@latest
else
    echo "benchstat is already installed."
fi



