#!/usr/bin/env bash
# Lambda のコードを更新してバージョンを公開し、live エイリアスを切り替える（docs/06 6.1）。
# 切り替える前のバージョンを <記録ファイル> に書き、失敗したときは rollback-lambda.sh で戻す。
# 使い方: deploy-lambda.sh <zip のディレクトリ> <記録ファイル>
# 環境変数: FUNCTION_PREFIX（例: saigai-prod）
set -euo pipefail

dir=$1
record=$2
: > "$record"
for fn in api ingest-jma ingest-quake matcher notifier; do
  name="${FUNCTION_PREFIX}-${fn}"
  prev=$(aws lambda get-alias --function-name "$name" --name live --query FunctionVersion --output text)
  echo "$name $prev" >> "$record"
  # 出力に ARN（アカウント ID を含む）を出さないよう、必要な値だけを取り出す
  ver=$(aws lambda update-function-code --function-name "$name" --zip-file "fileb://${dir}/${fn}.zip" --publish --query Version --output text)
  aws lambda wait published-version-active --function-name "$name" --qualifier "$ver"
  aws lambda update-alias --function-name "$name" --name live --function-version "$ver" --query FunctionVersion --output text > /dev/null
  echo "$name: live ${prev} -> ${ver}"
done
