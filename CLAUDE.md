# saigai-timeline

登録した地点の周辺で起きている災害情報（公式情報 + SNS の現場の声）を、1本のタイムラインにまとめて表示するサービス。
詳細な設計は `docs/` にある。設計判断に迷ったら、まず該当するドキュメントを確認すること。

## 技術スタック

> バージョンは実装開始時に確定し、`TBD` を書き換える。

| 領域 | 採用技術 | バージョン |
| --- | --- | --- |
| フロントエンド | Next.js（App Router, `output: 'export'` の静的出力）+ TypeScript | TBD |
| 地図 | MapLibre GL JS + 国土地理院タイル | TBD |
| Node.js | `web/.nvmrc` で固定、パッケージ管理は npm | TBD |
| バックエンド | Go（AWS Lambda, `provided.al2023`, arm64） | TBD（`api/go.mod`） |
| API | API Gateway HTTP API + JWT オーソライザ | - |
| データ | DynamoDB（オンデマンド）、S3 | - |
| 非同期処理 | SQS（+ DLQ）、EventBridge Scheduler | - |
| 認証 | Amazon Cognito（Authorization Code + PKCE、MFA / パスキー） | - |
| 配信 | S3 + CloudFront（OAC）+ WAF | - |
| 暗号化・秘密情報 | KMS（エンベロープ暗号化）、SSM Parameter Store（SecureString） | - |
| 通知 | SES、Web Push、Slack / Teams Webhook | - |
| IaC | Terraform（state は S3 に保存し、`use_lockfile` でロック） | TBD |
| CI/CD | GitHub Actions（AWS へは OIDC で認証） | - |
| リージョン | ap-northeast-1（CloudFront 用の証明書のみ us-east-1） | - |
| API 仕様 | OpenAPI（`api/openapi.yaml`）。Go は oapi-codegen、TS は openapi-typescript + openapi-fetch で生成 | TBD |

## テスト・動作検証

詳細は docs/08_test_strategy.md を参照。

| 層 | ツール |
| --- | --- |
| web: ユニット / コンポーネント | Vitest + React Testing Library（jsdom。MapLibre はモックに差し替える） |
| web: API モック | MSW（`web/src/mocks/`。シナリオ単位の fixture を用意し、Vitest・開発サーバー・E2E で共通利用） |
| web: E2E | Playwright（スマホのビューポート優先、Chromium + WebKit）、`@axe-core/playwright` |
| api: ユニット | `go test`（テーブル駆動）、`go-cmp`、Go 標準のファジング、`httptest.Server` で外部 API を代替 |
| api: 結合 | DynamoDB Local（`compose.yaml`）。ビルドタグ `integration` |
| api: 仕様との整合 | `kin-openapi` でレスポンスを OpenAPI スキーマと照合する |
| ローカル確認 | `api/cmd/local`（net/http で API を起動）、`api/cmd/replay`（電文 XML を流して災害時の流れを再現）、Mailpit |
| 負荷 | k6（Phase 2 以降） |

テストを書くときのルール:

- API を変更するときは、先に `api/openapi.yaml` を変更してから型を再生成する。生成したコードはコミットする
- CI から外部サービス（気象庁・P2P地震情報・国土地理院・SNS・地図タイル）を呼ばない。`testdata/` の fixture を使う
- 新しいエンドポイントを追加したら、認可テスト（他人のデータにアクセスできないこと）を必ず追加する
- ローカル専用の認証バイパスはビルドタグ `local` の中にだけ置く。Lambda のビルドに含めない
- フロントのテストは見える振る舞い（`getByRole` など）で書き、実装の詳細に依存しない

## 静的解析・脆弱性検査

| 種類 | 対象 | ツール | 実行タイミング |
| --- | --- | --- | --- |
| Lint / SAST | Go | `go vet`、`golangci-lint`（`gosec` を含む） | PR |
| Lint / SAST | TypeScript | ESLint（セキュリティ系ルールを含む）、`tsc --noEmit` | PR |
| SAST | 全体 | CodeQL（公開リポジトリの場合。非公開なら Semgrep OSS で代替） | PR |
| SCA（依存ライブラリ） | Go | `govulncheck` | PR / 週次 |
| SCA | npm | `npm audit`、`actions/dependency-review-action` | PR / 週次 |
| IaC | Terraform | `terraform fmt` / `validate`、`tflint`、`trivy config` | PR |
| 秘密情報 | 全体 | `gitleaks`（pre-commit と CI の両方） | コミット時 / PR |
| DAST | dev 環境 | OWASP ZAP ベースラインスキャン | 週次 |
| 依存の自動更新 | Go / npm / GitHub Actions / Terraform provider | Dependabot（Renovate への切り替えを検討中） | 週次 |

## リポジトリ構成（モノレポ）

```
web/           # Next.js
  src/mocks/   # MSW ハンドラと fixture
  e2e/         # Playwright
api/           # Go。cmd/<関数名>/ に Lambda ごとの main、internal/ にドメインロジック
  openapi.yaml # API 仕様（フロントとバックエンドの共通の正）
  cmd/local/   # ローカル API サーバー
  cmd/replay/  # 災害シナリオの再現
  cmd/opsctl/  # リリース・ロールバック・DR 切り替えの CLI
infra/         # Terraform。modules/ に再利用モジュール、envs/{dev,prod}/ に環境ごとの設定（東京・大阪の両リージョン）
docs/          # 設計ドキュメント
compose.yaml   # DynamoDB Local / Mailpit
.github/       # workflows / dependabot.yml / CODEOWNERS
```

## 守るべき制約

- **常時起動のリソースを使わない**: EC2 / ECS / RDS / NAT Gateway は使わない。Lambda は VPC の外で動かす。低コストを優先する
- **長期のアクセスキーを作らない**: CI は OIDC。人は MFA 必須の IAM ユーザー（アクセスキーなし）でログインし、`aws login` とロールの引き受けで一時クレデンシャルを使う（Identity Center は使わない）
- **最小権限**: Lambda には関数ごとに専用の IAM ロールを付ける。KMS の `Decrypt` 権限は `api` と `notifier` だけに与える
- **認可**: リクエストに含まれる `orgId` / `ownerId` は信用しない。必ずトークンの `sub` から `userId` と所属を引いて判定する。データのキーには `sub` ではなくアプリ独自の `userId`（ULID）を使う（DR で Cognito が切り替わっても紐付けられるように）。認可ロジックはミドルウェアに集約する
- **個人情報**: 正確な座標・住所・Webhook URL はアプリ層で暗号化してから保存する。ログには住所・座標・メールアドレス・トークンを出さない
- **ID**: ULID / UUID を使い、連番は使わない
- **フロントエンド**: `dangerouslySetInnerHTML` は使わない。長期トークンを `localStorage` に置かない
- **サプライチェーン**: 外部の GitHub Action はコミット SHA で固定する。`npm ci --ignore-scripts` を使い、ロックファイルをコミットする
- **冪等性**: 取り込み処理は `eventId` を使った条件付き書き込みで重複を排除する
- **新旧バージョンの共存**: カナリア中は新旧が同時に動き、ロールバックしてもデータは戻らない。属性・API の変更は「拡張 → 縮小」を別リリースで行い、新しいバージョンが書いたデータを古いバージョンが読めるようにする。書き込みには `writtenBy`（リリースの SHA）を付ける
- **マルチリージョン（DR）**: 東京（ap-northeast-1）がアクティブ、大阪（ap-northeast-3）がスタンバイ。リソースは両リージョンに作る。KMS はマルチリージョンキー、DynamoDB はグローバルテーブルを前提にする
- **DR の切り替え処理**: 大阪側の処理は東京の API に依存させない。切り替え・切り戻し・ロールバックは `opsctl` に実装し、CI・手元・watchdog から同じコードを呼ぶ。`--dry-run` を初期値にし、何度実行しても安全にする

## コマンド

> 実装開始後に追記する。

```sh
# ローカル環境（DynamoDB Local / Mailpit）
docker compose up -d

# api
go vet ./... && golangci-lint run && go test -race -cover ./... && govulncheck ./...
go test -tags=integration ./...      # 結合テスト（DynamoDB Local が必要）
go run ./cmd/local                   # ローカル API サーバー
go run ./cmd/replay <シナリオ名>     # 災害シナリオの再現
go run ./cmd/opsctl release status   # リリースの状態（両リージョン）
go run ./cmd/opsctl dr status        # DR の状態（両リージョン）

# web
npm run lint && npm run typecheck && npm test && npm run build
npm run dev:mock                     # MSW でモックした API で開発
npx playwright test                  # E2E（モック API）

# infra
terraform fmt -check -recursive infra && tflint --recursive --chdir=infra
```

## 設計ドキュメント

- コンセプト: @docs/01_concept_overview.md
- データモデル: @docs/04_data_model.md
- システム構成: @docs/05_system_architecture.md
- デプロイフロー: @docs/06_deploy_flow.md
- セキュリティ: @docs/07_security.md
- テスト・動作検証: @docs/08_test_strategy.md
- リリース方式（ブルーグリーン・ダークリリース・カナリア・シャドー・ロールバック）: docs/09_release_strategy.md（デプロイ・リリース操作を変更するときに参照）
- DR（大阪リージョンへの切り替え・バックアップ）: docs/10_disaster_recovery.md（インフラ・データ・認証を変更するときに参照）
- ユースケース: docs/02_use_case_scenarios.md、画面: docs/03_screens.md（必要なときに参照）
