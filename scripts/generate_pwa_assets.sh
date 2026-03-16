#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_IMAGE="${ROOT_DIR}/logo.png"
OUTPUT_DIR="${ROOT_DIR}/web/static/pwa"

if ! command -v ffmpeg >/dev/null 2>&1; then
    echo "ffmpeg is required to generate PWA icons." >&2
    exit 1
fi

if [ ! -f "${SOURCE_IMAGE}" ]; then
    echo "logo.png not found at repository root." >&2
    exit 1
fi

mkdir -p "${OUTPUT_DIR}"

crop_filter="crop=min(in_w\\,in_h):min(in_w\\,in_h)"

generate_icon() {
    local size="$1"
    local filename="$2"

    ffmpeg -y -i "${SOURCE_IMAGE}" \
        -vf "${crop_filter},scale=${size}:${size}:flags=lanczos" \
        -frames:v 1 \
        "${OUTPUT_DIR}/${filename}" \
        >/dev/null 2>&1
}

generate_icon 64 "favicon-64.png"
generate_icon 32 "favicon-32.png"
generate_icon 16 "favicon-16.png"
generate_icon 180 "apple-touch-icon.png"
generate_icon 192 "icon-192.png"
generate_icon 512 "icon-512.png"
generate_icon 512 "maskable-512.png"

ffmpeg -y -i "${SOURCE_IMAGE}" \
    -vf "${crop_filter},scale=64:64:flags=lanczos" \
    -frames:v 1 \
    "${OUTPUT_DIR}/favicon.ico" \
    >/dev/null 2>&1

echo "Generated PWA icons in ${OUTPUT_DIR}"
