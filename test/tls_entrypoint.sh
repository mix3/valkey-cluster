#!/bin/bash
set -e

# 0. openssl コマンドが無い場合のみ自動インストール
if ! command -v openssl &> /dev/null; then
  echo "openssl not found. Installing..."
  if command -v apt-get &> /dev/null; then
    apt-get update && apt-get install -y openssl
  elif command -v apk &> /dev/null; then
    apk add --no-cache openssl
  else
    echo "Error: Neither apt-get nor apk found. Cannot install openssl."
    exit 1
  fi
fi

# 1. 自分の最新IPを取得
MY_IP=$(hostname -I | awk '{print $1}')
TLS_DIR="/tls"
mkdir -p ${TLS_DIR}

# 2. コンテナローカルな共通CAを作成
openssl req -x509 -nodes -days 36500 -newkey rsa:2048 \
  -keyout "${TLS_DIR}/ca.key" \
  -out "${TLS_DIR}/ca.crt" \
  -subj "/CN=Valkey-Local-CA"

# 3. コンテナ用の「共通」秘密鍵と証明書を1セットだけ作成
# （すべてのポートでこの証明書を使い回します）
openssl req -new -nodes -newkey rsa:2048 \
  -keyout "${TLS_DIR}/node.key" \
  -out "/tmp/node.csr" \
  -subj "/CN=${MY_IP}" \
  -addext "subjectAltName=IP:${MY_IP},DNS:localhost"

openssl x509 -req -in "/tmp/node.csr" \
  -CA "${TLS_DIR}/ca.crt" \
  -CAkey "${TLS_DIR}/ca.key" \
  -CAcreateserial \
  -out "${TLS_DIR}/node.crt" \
  -days 1 \
  -extfile <(echo "subjectAltName=IP:${MY_IP},DNS:localhost")

exec /launcher "$@"
