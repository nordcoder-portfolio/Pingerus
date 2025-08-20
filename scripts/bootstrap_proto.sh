#!/usr/bin/env bash
set -euo pipefail

# --- pkgs for apt/yum ---
if command -v apt-get &>/dev/null; then
  sudo apt-get update -qq
  sudo apt-get install -y --no-install-recommends \
    curl build-essential protobuf-compiler git
elif command -v yum &>/dev/null; then
  sudo yum install -y \
    curl gcc make protobuf-compiler git
else
  echo "Неизвестный пакетный менеджер. Поддерживаются apt-get и yum."
  exit 1
fi

if ! command -v protoc &>/dev/null; then
  echo "❌ protoc не установлен после установки protobuf-compiler"
  exit 1
fi
echo "✔ protoc version: $(protoc --version)"

# --- check go ---
if ! command -v go &>/dev/null; then
  echo "❌ Go не найден. Установите Go >= 1.21: https://go.dev/dl/"
  exit 1
fi

export PATH="$HOME/go/bin:$PATH"

PLUGINS=(
  google.golang.org/protobuf/cmd/protoc-gen-go
  google.golang.org/grpc/cmd/protoc-gen-go-grpc
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
  github.com/envoyproxy/protoc-gen-validate
)
for mod in "${PLUGINS[@]}"; do
  BIN=$(basename "${mod}")
  if ! command -v "${BIN}" &>/dev/null; then
    echo "▶ Installing ${BIN}..."
    go install "${mod}@latest"
  else
    echo "✔ ${BIN} already installed"
  fi
done

GOOGLEAPIS_DIR="third_party/googleapis"
if [ ! -d "${GOOGLEAPIS_DIR}" ]; then
  echo "▶ Cloning googleapis into ${GOOGLEAPIS_DIR}…"
  git clone --depth=1 https://github.com/googleapis/googleapis.git "${GOOGLEAPIS_DIR}"
else
  echo "✔ googleapis already present"
fi

PGV_DIR="third_party/protoc-gen-validate"
if [ ! -d "${PGV_DIR}" ]; then
  echo "▶ Cloning protoc-gen-validate into ${PGV_DIR}…"
  git clone --depth=1 https://github.com/bufbuild/protoc-gen-validate.git "${PGV_DIR}"
else
  echo "✔ protoc-gen-validate (protos) already present"
fi

echo "──────────────────────────────────────────────────────────"
echo "✔ protoc version: $(protoc --version)"
echo "✔ installed plugins:"
for bin in protoc-gen-go protoc-gen-go-grpc protoc-gen-grpc-gateway protoc-gen-openapiv2 protoc-gen-validate; do
  echo "  - $bin: $(command -v $bin)"
done
echo "──────────────────────────────────────────────────────────"
