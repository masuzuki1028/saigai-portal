#!/usr/bin/env bash
# 画面の静的出力（web/out）を S3 に置き、CloudFront のキャッシュを消す（docs/06 6.2）。
# 使い方: deploy-web.sh <out のディレクトリ>
# 環境変数: WEB_BUCKET、DISTRIBUTION_ID
set -euo pipefail

out=$1
dest="s3://${WEB_BUCKET}"
# 1. ハッシュ付きのファイルは長くキャッシュする（先に置き、新しい HTML から参照できるようにする）
aws s3 sync "$out/_next/static" "$dest/_next/static" --only-show-errors --cache-control "public,max-age=31536000,immutable"
# 2. HTML などは毎回確かめる
aws s3 sync "$out" "$dest" --only-show-errors --exclude "_next/static/*" --exclude "*.mjs" --cache-control "public,max-age=0,must-revalidate"
# 3. MapLibre の Web Worker（.mjs）は text/javascript で配信する（違うと地図の図形が描画されない）
while IFS= read -r f; do
  aws s3 cp "$out/$f" "$dest/$f" --only-show-errors --content-type "text/javascript" --cache-control "public,max-age=0,must-revalidate"
done < <(cd "$out" && find . -name '*.mjs' | sed 's|^\./||')
# 4. 古いファイルを消す（置いたファイルは上で送り済みのため、ここでは大きさが違うものだけ送り直す）
aws s3 sync "$out" "$dest" --only-show-errors --delete --size-only --exclude "*.mjs" --cache-control "public,max-age=0,must-revalidate"
aws cloudfront create-invalidation --distribution-id "$DISTRIBUTION_ID" --paths "/*" --query Invalidation.Id --output text > /dev/null
echo "web deployed"
