#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "usage: $0 <amd64|arm64> <itb-binary>" >&2
    exit 2
fi

case "$1" in
    amd64) image="quay.io/pypa/manylinux_2_28_x86_64" ;;
    arm64) image="quay.io/pypa/manylinux_2_28_aarch64" ;;
    *) echo "unsupported architecture: $1" >&2; exit 2 ;;
esac

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
cp "$2" "$work_dir/itb"
chmod 0755 "$work_dir/itb"

# Tiny valid fixtures are generated here so production Go tests keep their
# fixture-free convention while this release smoke test exercises real codecs.
printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9WlKDwAAAABJRU5ErkJggg==' | base64 --decode > "$work_dir/sample.png"
printf '%s' '/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAX/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAF//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABBQJ//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAwEBPwF//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAgEBPwF//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQAGPwJ//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPyF//9k=' | base64 --decode > "$work_dir/sample.jpg"

docker run --rm -v "$work_dir:/workspace" -w /workspace "$image" sh -ec '
    ./itb compress -i sample.png -o output.png -q 80
    test -s output.png
    ./itb compress -i sample.jpg -o output.jpg -q 80
    test -s output.jpg
'
