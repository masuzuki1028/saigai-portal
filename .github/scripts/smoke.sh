#!/usr/bin/env bash
# デプロイ後のスモークテスト（docs/06 5.2、docs/08 4.3）。API の /health と主要な画面が 200 を返すこと。
# 使い方: smoke.sh <画面の URL（例: https://saigai-portal.example.com）>
set -euo pipefail

base=$1
fail=0
for path in /api/health / /login /auth/callback /terms; do
  code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 --retry 3 --retry-all-errors "${base}${path}") || code=000
  echo "${path}: ${code}"
  [[ $code == 200 ]] || fail=1
done
exit $fail
