#!/usr/bin/env bash
# 生成 mtapi (MT4/MT5) Go gRPC 桩代码
# 不修改 /opt/alfq/gprc/ 中的官方 proto，通过临时 wrapper 改写 go_package
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
GPRC_DIR="$ROOT/gprc"
GEN_DIR="$ROOT/backend/go/gen"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ ! -f "$GPRC_DIR/mt4.proto" || ! -f "$GPRC_DIR/mt5.proto" ]]; then
  echo "ERROR: $GPRC_DIR/mt4.proto or mt5.proto missing" >&2
  exit 1
fi

# 1. 复制官方 proto 到临时 wrapper
cp "$GPRC_DIR/mt4.proto" "$TMP_DIR/mt4.proto"
cp "$GPRC_DIR/mt5.proto" "$TMP_DIR/mt5.proto"

# 2. 改写 go_package（不动官方原文件）
sed -i 's|git.mtapi.io/root/grpc-proto.git/mt4/go|github.com/alfq/backend/go/gen/mt4|g' "$TMP_DIR/mt4.proto"
sed -i 's|git.mtapi.io/root/grpc-proto.git/mt5/go|github.com/alfq/backend/go/gen/mt5|g' "$TMP_DIR/mt5.proto"

# 3. 生成 buf 配置
cat > "$TMP_DIR/buf.yaml" <<'EOF'
version: v1
breaking:
  use:
    - FILE
lint:
  use:
    - DEFAULT
  except:
    - PACKAGE_VERSION_SUFFIX
    - SERVICE_SUFFIX
    - ENUM_VALUE_PREFIX
    - RPC_REQUEST_STANDARD_NAME
    - RPC_RESPONSE_STANDARD_NAME
    - ENUM_ZERO_VALUE_SUFFIX
EOF

cat > "$TMP_DIR/buf.gen.yaml" <<EOF
version: v1
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: $GEN_DIR
    opt:
      - paths=source_relative
  - plugin: buf.build/grpc/go
    out: $GEN_DIR
    opt:
      - paths=source_relative
EOF

# 4. 调用 buf generate (uses remote plugins from buf.build)
mkdir -p "$GEN_DIR"
buf generate "$TMP_DIR" --template "$TMP_DIR/buf.gen.yaml"

# 5. 整理产物到 mt4/ mt5/ 子目录
mkdir -p "$GEN_DIR/mt4" "$GEN_DIR/mt5"
mv -f "$GEN_DIR/mt4.pb.go"      "$GEN_DIR/mt4/mt4.pb.go"
mv -f "$GEN_DIR/mt4_grpc.pb.go" "$GEN_DIR/mt4/mt4_grpc.pb.go"
mv -f "$GEN_DIR/mt5.pb.go"      "$GEN_DIR/mt5/mt5.pb.go"
mv -f "$GEN_DIR/mt5_grpc.pb.go" "$GEN_DIR/mt5/mt5_grpc.pb.go"

echo "OK: generated"
echo "  $GEN_DIR/mt4/{mt4.pb.go,mt4_grpc.pb.go}"
echo "  $GEN_DIR/mt5/{mt5.pb.go,mt5_grpc.pb.go}"
