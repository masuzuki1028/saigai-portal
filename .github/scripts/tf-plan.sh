#!/usr/bin/env bash
# terraform plan を実行し、追加・変更・削除の件数だけをジョブの要約に出す。
# 公開リポジトリの Actions のログは誰でも読めるため、plan の全文は出さない（docs/06 5章）。
#
# 使い方: tf-plan.sh <ディレクトリ> [terraform plan に渡す引数...]
# 環境変数:
#   TF_STATE_BUCKET  state のバケット（必須。Secrets）
#   TF_VARS          terraform.tfvars の内容（任意。Secrets）
# 出力（GITHUB_OUTPUT）:
#   count  差分のあるリソースの数
#   hash   差分（リソースと操作）のハッシュ。承認の前後で plan が変わっていないかの確認に使う
set -euo pipefail

dir=$1
shift
cd "$dir"

if [[ -n "${TF_VARS:-}" ]]; then
  printf '%s\n' "$TF_VARS" > terraform.tfvars
  # 公開したくない値（アカウント ID・ドメイン・メールアドレス）は、エラーの表示でも伏せる
  while IFS= read -r v; do
    if [[ -n $v ]]; then echo "::add-mask::$v"; fi
  done < <(grep -E '^[[:space:]]*(account_id|root_domain|alarm_email)[[:space:]]*=' terraform.tfvars | sed -E 's/^[^=]*=[[:space:]]*"?([^"]*)"?[[:space:]]*$/\1/')
fi

log=$(mktemp)
# 失敗したときはエラーの部分だけを出す
run() {
  if ! "$@" > "$log" 2>&1; then
    grep -E '^(Error|│|╷|╵)' "$log" >&2 || tail -n 5 "$log" >&2
    exit 1
  fi
}

run terraform init -input=false -no-color -backend-config="bucket=${TF_STATE_BUCKET}"
run terraform plan -input=false -no-color -out=tfplan "$@"

changes=$(terraform show -json tfplan | jq -c '[.resource_changes[]? | select(.change.actions != ["no-op"]) | {address, actions: .change.actions}] | sort_by(.address)')
count=$(jq 'length' <<< "$changes")
{
  echo "### terraform plan（${dir}）"
  if [[ $count == 0 ]]; then
    echo "差分なし"
  else
    jq -r 'group_by(.actions | join(",")) | map("- \(.[0].actions | join(",")): \(length) 件") | .[]' <<< "$changes"
  fi
} >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "count=${count}" >> "$GITHUB_OUTPUT"
  echo "hash=$(sha256sum <<< "$changes" | cut -c1-64)" >> "$GITHUB_OUTPUT"
fi
