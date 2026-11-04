#!/usr/bin/env bash
# Lambda の関数を arm64 でビルドし、関数ごとの zip を作る（docs/06 6.1）。
# 使い方: build-lambda.sh <出力先のディレクトリ>   （api/ で実行する）
set -euo pipefail

out=$1
mkdir -p "$out"
for fn in api ingest-jma ingest-quake matcher notifier; do
  work=$(mktemp -d)
  # vcs.revision（コミット）が埋め込まれ、書き込みの writtenBy に使われる
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$work/bootstrap" "./cmd/$fn"
  (cd "$work" && zip -q -X "$out/$fn.zip" bootstrap)
  rm -rf "$work"
  echo "built $fn ($(du -h "$out/$fn.zip" | cut -f1))"
done
