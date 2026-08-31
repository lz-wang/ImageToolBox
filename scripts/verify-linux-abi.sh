#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "usage: $0 <bins-dir>" >&2
    exit 2
fi

bins_dir="$1"
baseline="2.28"
binaries=(pngquant oxipng cjpeg-static djpeg-static)

allowed_dependency() {
    case "$1" in
        libc.so.6|libm.so.6|libgcc_s.so.1|libpthread.so.0|libdl.so.2|librt.so.1) return 0 ;;
        *) return 1 ;;
    esac
}

check_glibc() {
    local binary="$1"
    local versions=()
    mapfile -t versions < <(readelf --version-info "$binary" | grep -oE 'GLIBC_[0-9]+(\.[0-9]+)+' | sed 's/^GLIBC_//' | sort -Vu || true)
    if [[ ${#versions[@]} -eq 0 ]]; then
        printf '%-20s no GLIBC symbol dependency\n' "$(basename "$binary")"
        return
    fi
    local max_version="${versions[${#versions[@]} - 1]}"
    if [[ "$(printf '%s\n%s\n' "$max_version" "$baseline" | sort -V | tail -1)" != "$baseline" ]]; then
        echo "ERROR: $binary requires GLIBC_$max_version; baseline is GLIBC_$baseline" >&2
        exit 1
    fi
    printf '%-20s GLIBC <= %s\n' "$(basename "$binary")" "$max_version"
}

check_dependencies() {
    local binary="$1"
    local dependency
    while IFS= read -r dependency; do
        if ! allowed_dependency "$dependency"; then
            echo "ERROR: $binary requires unsupported shared library: $dependency" >&2
            exit 1
        fi
    done < <(readelf -d "$binary" | awk -F'[][]' '/NEEDED/ { print $2 }')
}

for name in "${binaries[@]}"; do
    binary="$bins_dir/$name"
    if [[ ! -x "$binary" ]]; then
        echo "missing binary: $binary" >&2
        exit 1
    fi
    check_glibc "$binary"
    check_dependencies "$binary"
done
