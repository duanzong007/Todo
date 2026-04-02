#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ANDROID_DIR="${ROOT_DIR}/android-shell/android/app/src/main/res"
SOURCE_LOGO="${ROOT_DIR}/logo.png"

if ! command -v ffmpeg >/dev/null 2>&1; then
    echo "ffmpeg is required to generate Android assets." >&2
    exit 1
fi

if [ ! -f "${SOURCE_LOGO}" ]; then
    echo "logo.png not found at repository root." >&2
    exit 1
fi

bg="#f6ecdc"
crop="crop=min(in_w\\,in_h):min(in_w\\,in_h)"

make_launcher() {
    local size="$1"
    local target="$2"
    ffmpeg -y -i "${SOURCE_LOGO}" \
        -vf "${crop},scale=${size}:${size}:flags=lanczos" \
        -frames:v 1 \
        "${target}" \
        >/dev/null 2>&1
}

make_foreground() {
    local size="$1"
    local inner="$2"
    local target="$3"
    ffmpeg -y -f lavfi -i "color=color=0x00000000:size=${size}x${size}" -i "${SOURCE_LOGO}" \
        -filter_complex "[1:v]${crop},scale=${inner}:${inner}:flags=lanczos[logo];[0:v][logo]overlay=(W-w)/2:(H-h)/2:format=auto" \
        -frames:v 1 \
        "${target}" \
        >/dev/null 2>&1
}

make_splash() {
    local width="$1"
    local height="$2"
    local logo_size="$3"
    local target="$4"
    local title_size="$5"
    local title_y="$6"
    local logo_y="$7"
    SWIFT_WIDTH="${width}" \
    SWIFT_HEIGHT="${height}" \
    SWIFT_LOGO_SIZE="${logo_size}" \
    SWIFT_TARGET="${target}" \
    SWIFT_LOGO_PATH="${SOURCE_LOGO}" \
    SWIFT_TITLE_SIZE="${title_size}" \
    SWIFT_TITLE_TOP="${title_y}" \
    SWIFT_LOGO_TOP="${logo_y}" \
    /usr/bin/swift - <<'SWIFT' >/dev/null 2>&1
import AppKit
import Foundation

let env = ProcessInfo.processInfo.environment

guard
    let width = Double(env["SWIFT_WIDTH"] ?? ""),
    let height = Double(env["SWIFT_HEIGHT"] ?? ""),
    let logoSize = Double(env["SWIFT_LOGO_SIZE"] ?? ""),
    let titleSize = Double(env["SWIFT_TITLE_SIZE"] ?? ""),
    let titleTop = Double(env["SWIFT_TITLE_TOP"] ?? ""),
    let logoTop = Double(env["SWIFT_LOGO_TOP"] ?? ""),
    let target = env["SWIFT_TARGET"],
    let logoPath = env["SWIFT_LOGO_PATH"],
    let logoImage = NSImage(contentsOfFile: logoPath)
else {
    exit(8)
}

let pixelWidth = Int(width)
let pixelHeight = Int(height)
let canvasRect = NSRect(x: 0, y: 0, width: width, height: height)

guard
    let rep = NSBitmapImageRep(
        bitmapDataPlanes: nil,
        pixelsWide: pixelWidth,
        pixelsHigh: pixelHeight,
        bitsPerSample: 8,
        samplesPerPixel: 4,
        hasAlpha: true,
        isPlanar: false,
        colorSpaceName: .deviceRGB,
        bytesPerRow: 0,
        bitsPerPixel: 0
    )
else {
    exit(8)
}

rep.size = NSSize(width: width, height: height)

NSGraphicsContext.saveGraphicsState()
guard let context = NSGraphicsContext(bitmapImageRep: rep) else {
    exit(8)
}
NSGraphicsContext.current = context

NSColor(calibratedRed: 246.0 / 255.0, green: 236.0 / 255.0, blue: 220.0 / 255.0, alpha: 1).setFill()
canvasRect.fill()

let paragraph = NSMutableParagraphStyle()
paragraph.alignment = .center

let font = NSFont(name: "Georgia-Bold", size: titleSize) ?? NSFont.systemFont(ofSize: titleSize, weight: .bold)
let attrs: [NSAttributedString.Key: Any] = [
    .font: font,
    .foregroundColor: NSColor(calibratedRed: 44.0 / 255.0, green: 36.0 / 255.0, blue: 29.0 / 255.0, alpha: 1),
    .paragraphStyle: paragraph
]

let title = NSString(string: "Todo")
let titleHeight = titleSize * 1.35
let titleRect = NSRect(
    x: 0,
    y: height - titleTop - titleHeight,
    width: width,
    height: titleHeight
)
title.draw(in: titleRect, withAttributes: attrs)

let logoRect = NSRect(
    x: (width - logoSize) / 2.0,
    y: height - logoTop - logoSize,
    width: logoSize,
    height: logoSize
)
logoImage.draw(in: logoRect)

context.flushGraphics()
NSGraphicsContext.restoreGraphicsState()

let data = rep.representation(using: .png, properties: [:])
try data?.write(to: URL(fileURLWithPath: target))
SWIFT
}

make_launcher 48   "${ANDROID_DIR}/mipmap-mdpi/ic_launcher.png"
make_launcher 72   "${ANDROID_DIR}/mipmap-hdpi/ic_launcher.png"
make_launcher 96   "${ANDROID_DIR}/mipmap-xhdpi/ic_launcher.png"
make_launcher 144  "${ANDROID_DIR}/mipmap-xxhdpi/ic_launcher.png"
make_launcher 192  "${ANDROID_DIR}/mipmap-xxxhdpi/ic_launcher.png"

make_launcher 48   "${ANDROID_DIR}/mipmap-mdpi/ic_launcher_round.png"
make_launcher 72   "${ANDROID_DIR}/mipmap-hdpi/ic_launcher_round.png"
make_launcher 96   "${ANDROID_DIR}/mipmap-xhdpi/ic_launcher_round.png"
make_launcher 144  "${ANDROID_DIR}/mipmap-xxhdpi/ic_launcher_round.png"
make_launcher 192  "${ANDROID_DIR}/mipmap-xxxhdpi/ic_launcher_round.png"

make_foreground 108 72   "${ANDROID_DIR}/mipmap-mdpi/ic_launcher_foreground.png"
make_foreground 162 108  "${ANDROID_DIR}/mipmap-hdpi/ic_launcher_foreground.png"
make_foreground 216 144  "${ANDROID_DIR}/mipmap-xhdpi/ic_launcher_foreground.png"
make_foreground 324 216  "${ANDROID_DIR}/mipmap-xxhdpi/ic_launcher_foreground.png"
make_foreground 432 288  "${ANDROID_DIR}/mipmap-xxxhdpi/ic_launcher_foreground.png"

make_splash 320 480 148   "${ANDROID_DIR}/drawable/splash.png"               28 52   188
make_splash 320 480 148   "${ANDROID_DIR}/drawable-port-mdpi/splash.png"     28 52   188
make_splash 480 800 220   "${ANDROID_DIR}/drawable-port-hdpi/splash.png"     42 84   316
make_splash 720 1280 320  "${ANDROID_DIR}/drawable-port-xhdpi/splash.png"    62 132  502
make_splash 960 1600 420  "${ANDROID_DIR}/drawable-port-xxhdpi/splash.png"   82 166  620
make_splash 1280 1920 520 "${ANDROID_DIR}/drawable-port-xxxhdpi/splash.png"  108 206 760

make_splash 480 320 124   "${ANDROID_DIR}/drawable-land-mdpi/splash.png"     30 34   128
make_splash 800 480 180   "${ANDROID_DIR}/drawable-land-hdpi/splash.png"     42 48   196
make_splash 1280 720 250  "${ANDROID_DIR}/drawable-land-xhdpi/splash.png"    62 68   292
make_splash 1600 960 320  "${ANDROID_DIR}/drawable-land-xxhdpi/splash.png"   78 84   390
make_splash 1920 1280 390 "${ANDROID_DIR}/drawable-land-xxxhdpi/splash.png"  92 108  520

echo "Generated Android launcher icons and splash assets."
