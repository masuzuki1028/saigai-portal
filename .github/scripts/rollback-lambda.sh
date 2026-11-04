#!/usr/bin/env bash
# deploy-lambda.sh が記録した切り替え前のバージョンに、live エイリアスを戻す（docs/06 7章）。
# 使い方: rollback-lambda.sh <記録ファイル>
set -euo pipefail

record=$1
[[ -s $record ]] || exit 0
while read -r name prev; do
  # 最初のデプロイの前（$LATEST を指していた）は戻す先がない
  if [[ $prev == "\$LATEST" ]]; then
    echo "$name: 戻す先のバージョンがありません（最初のデプロイ）"
    continue
  fi
  aws lambda update-alias --function-name "$name" --name live --function-version "$prev" --query FunctionVersion --output text > /dev/null
  echo "$name: live -> ${prev}"
done < "$record"
