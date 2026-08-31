#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "usage: $0 <amd64|arm64> <itb-binary>" >&2
    exit 2
fi

case "$1" in
    amd64) image="quay.io/pypa/manylinux_2_28_x86_64@sha256:0bf9db09181f36be8cf0628332abb5d31855b7d0f372faa776f492ccd9d3100d" ;;
    arm64) image="quay.io/pypa/manylinux_2_28_aarch64@sha256:d3524021bb8b15d2258481be1ea46f1dfa8a9c7fc09bec43a6c5407945f9dc11" ;;
    *) echo "unsupported architecture: $1" >&2; exit 2 ;;
esac

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
cp "$2" "$work_dir/itb"
chmod 0755 "$work_dir/itb"

# Generate transparent, auditable fixtures with Go's standard codecs. This
# keeps production tests fixture-free while exercising the real native codecs.
go run ./scripts/generate-compress-fixtures.go "$work_dir"

docker run --rm -v "$work_dir:/workspace" -w /workspace "$image" sh -ec '
    ./itb compress -i sample.png -o output.png -q 80
    test -s output.png
    ./itb compress -i sample.jpg -o output.jpg -q 80
    test -s output.jpg
'
