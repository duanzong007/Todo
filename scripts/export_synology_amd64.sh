#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist/synology-amd64"
DEPLOY_DIR="$ROOT_DIR/deploy/synology"
IMAGE_NAME="todo-app"
IMAGE_TAG="synology-amd64"
IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"
IMAGE_TAR="${DIST_DIR}/${IMAGE_NAME}-${IMAGE_TAG}.tar"
BUNDLE_TAR="${ROOT_DIR}/dist/todo-synology-amd64-bundle.tar.gz"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker 未安装，无法导出群晖镜像。" >&2
  exit 1
fi

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx 不可用，无法构建 linux/amd64 镜像。" >&2
  exit 1
fi

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

cp "$DEPLOY_DIR/docker-compose.with-db.yml" "$DIST_DIR/docker-compose.with-db.yml"
cp "$DEPLOY_DIR/docker-compose.external-db.yml" "$DIST_DIR/docker-compose.external-db.yml"
cp "$DEPLOY_DIR/.env.example" "$DIST_DIR/.env.example"
cp "$DEPLOY_DIR/README.md" "$DIST_DIR/README.md"

echo "==> 构建 linux/amd64 镜像: ${IMAGE_REF}"
docker buildx build \
  --platform linux/amd64 \
  --tag "$IMAGE_REF" \
  --output "type=docker,dest=${IMAGE_TAR}" \
  "$ROOT_DIR"

echo "==> 打包群晖部署 bundle"
tar -C "$DIST_DIR" -czf "$BUNDLE_TAR" .

cat <<EOF
完成。

镜像导出文件:
  ${IMAGE_TAR}

群晖 bundle:
  ${BUNDLE_TAR}

导入到群晖时，优先使用:
  1. 导入镜像文件 ${IMAGE_NAME}-${IMAGE_TAG}.tar
  2. 使用同目录下的 docker-compose.with-db.yml 或 docker-compose.external-db.yml
EOF
