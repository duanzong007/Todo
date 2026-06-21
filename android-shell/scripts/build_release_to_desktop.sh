#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHELL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ANDROID_DIR="${SHELL_DIR}/android"
GRADLE_FILE="${ANDROID_DIR}/app/build.gradle"
APK_SOURCE="${ANDROID_DIR}/app/build/outputs/apk/release/app-release.apk"
CHANGELOG_FILE="${SHELL_DIR}/release-changelog.txt"
DESKTOP_DIR="${HOME}/Desktop"

read_gradle_value() {
  local key="$1"
  awk -v key="${key}" '$1 == key { gsub(/\"/, "", $2); print $2; exit }' "${GRADLE_FILE}"
}

read_changelog() {
  if [[ -n "${ANDROID_UPDATE_CHANGELOG:-}" ]]; then
    printf '%s' "${ANDROID_UPDATE_CHANGELOG}"
    return
  fi

  if [[ ! -f "${CHANGELOG_FILE}" ]]; then
    printf '%s' "请填写安卓壳更新说明"
    return
  fi

  awk '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
      if (output != "") output = output "|"
      output = output $0
    }
    END { print output }
  ' "${CHANGELOG_FILE}"
}

sha256_file() {
  local file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file}" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
    return
  fi
  echo "缺少 shasum 或 sha256sum，无法计算 SHA256" >&2
  exit 1
}

VERSION_NAME="$(read_gradle_value versionName)"
VERSION_CODE="$(read_gradle_value versionCode)"

if [[ -z "${VERSION_NAME}" || -z "${VERSION_CODE}" ]]; then
  echo "无法从 ${GRADLE_FILE} 读取安卓版本号" >&2
  exit 1
fi

if [[ "$(uname -s)" == "Darwin" ]] && [[ -x /usr/libexec/java_home ]]; then
  export JAVA_HOME="$(/usr/libexec/java_home -v 21)"
fi

mkdir -p "${DESKTOP_DIR}"

echo "正在构建 Todo 安卓壳 v${VERSION_NAME} (${VERSION_CODE})..."
"${ANDROID_DIR}/gradlew" -p "${ANDROID_DIR}" :app:assembleRelease

if [[ ! -f "${APK_SOURCE}" ]]; then
  echo "构建完成，但没有找到 APK：${APK_SOURCE}" >&2
  exit 1
fi

APK_TARGET="${DESKTOP_DIR}/Todo-android-shell-v${VERSION_NAME}-release.apk"
cp -f "${APK_SOURCE}" "${APK_TARGET}"

APK_SHA256="$(sha256_file "${APK_TARGET}")"
CHANGELOG="$(read_changelog)"

echo
echo "APK 已生成：${APK_TARGET}"
echo
echo "ANDROID_UPDATE_VERSION_NAME=${VERSION_NAME}"
echo "ANDROID_UPDATE_VERSION_CODE=${VERSION_CODE}"
echo "ANDROID_UPDATE_SHA256=${APK_SHA256}"
echo "ANDROID_UPDATE_CHANGELOG=${CHANGELOG}"
