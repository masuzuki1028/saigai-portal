# infra/bootstrap

最初の1回だけ手元から apply する Terraform。以後のインフラの変更は CI（`infra/envs/prod`）で行う。

| 作るもの | 内容 |
| --- | --- |
| state のバケット | `saigai-tfstate-<アカウントID>`（バージョニング・暗号化・公開禁止・TLS 必須） |
| GitHub の OIDC プロバイダ | GitHub Actions が長期のキーなしで AWS を使うため |
| `saigai-gha-plan` | CI の `terraform plan` 用。読み取り専用（本番のデータは読めない） |
| `saigai-gha-deploy` | CI のデプロイ用。GitHub の Environment「prod」の承認後だけ。IAM は `saigai-prod-*` だけで、作るロールには権限の上限（`saigai-prod-boundary`）が必須 |
| `saigai-readonly` | 運営者の普段使い（Claude Code の plan もこれ）。読み取り専用（本番のデータは読めない） |
| `saigai-admin` | 運営者の最初の構築・緊急時用 |
| 運営者の IAM ユーザーのポリシー | 上の2つのロールの引き受けと、自分のパスワード・MFA の管理だけ |

## 前提

- AWS CLI 2.3x 以上（`aws login` が使えること）
- Terraform 1.16.5（`infra/.terraform-version`。tfenv なら自動で使われる）
- 運営者用の IAM ユーザーをコンソールで作成済み（パスワード＋MFA。**アクセスキーは作らない**）。名前は本名と結び付かないもの（例: `saigai-operator`）にする（ARN やログに出るため）

## 手順

### 1. IAM ユーザーに一時的に管理者権限を付ける

コンソールで、運営者用の IAM ユーザーに `AdministratorAccess` を付ける（手順 5 で外す）。

### 2. ログインする

```sh
aws login --profile saigai          # ブラウザで IAM ユーザー＋MFA でログイン
aws sts get-caller-identity --profile saigai
```

### 3. 変数を用意して apply する

```sh
cd infra/bootstrap
cp terraform.tfvars.example terraform.tfvars   # account_id と operator_user_name を入れる（.gitignore 済み）
# GitHub Actions の OIDC プロバイダがすでにあるか確認する。あれば terraform.tfvars に create_github_oidc_provider = false を書く
aws iam list-open-id-connect-providers --query "OpenIDConnectProviderList[?ends_with(Arn,'token.actions.githubusercontent.com')]"
export AWS_PROFILE=saigai
terraform init
terraform plan -out=tfplan        # 作られるものを確認する
terraform apply tfplan
terraform output -raw aws_config  # ~/.aws/config に追記する内容
```

### 4. state を S3 に移す

```sh
mv backend.tf.example backend.tf
terraform init -migrate-state -backend-config="bucket=$(terraform output -raw state_bucket)"
rm -f terraform.tfstate terraform.tfstate.backup tfplan   # 移したら手元の state は消す
```

`backend.tf` はコミットしてよい（バケット名は書いていない）。

### 5. 管理者権限を外し、ロールで操作する

1. コンソールで、IAM ユーザーから `AdministratorAccess` を外す（`saigai-operator` のポリシーだけになる）
2. `terraform output -raw aws_config` の内容を `~/.aws/config` に追記する
3. 確認する

```sh
aws sts get-caller-identity --profile saigai-readonly
aws sts get-caller-identity --profile saigai-admin
```

`aws login` の認証情報には「MFA で認証済み」の印が付かないため、MFA 必須のロールは `mfa_serial` を設定して、
引き受けるときに認証アプリ（TOTP）のコードを入力する（2026-10 確認）。CLI は引き受けた認証情報を保存するので、
端末で一度コードを入力すれば、セッションの間（1 時間）は Claude Code からも同じプロファイルを使える。
`aws login` の認証情報からロールを引き受けるのは「ロールの連鎖」になるため、セッションは最長 1 時間（`duration_seconds = 3600`）。

```sh
aws sts get-caller-identity --profile saigai-readonly   # 「Enter MFA code」と聞かれたら、認証アプリのコードを入力する
```

認証アプリの MFA がない（パスキーだけの）場合は、コンソールで認証アプリを追加で登録する。

Terraform は MFA のコードを入力できず、CLI が保存したセッションも使えないため、Terraform では
`credential_process` で CLI からセッションを借りる `-tf` のプロファイルを使う（先に CLI で上のコマンドを実行してコードを入力しておく）。

| 用途 | AWS CLI | Terraform |
| --- | --- | --- |
| 読み取り（plan など） | `saigai-readonly` | `saigai-readonly-tf` |
| 変更（apply） | `saigai-admin` | `saigai-admin-tf` |

### 6. GitHub に登録する

公開リポジトリの Actions のログは誰でも読めるため、アカウント ID を含む値は **Variables ではなく Secrets** に置く（Secrets はログで伏せられる）。

| 場所 | 名前 | 値 |
| --- | --- | --- |
| Settings → Secrets and variables → Actions → **Secrets**（Repository secrets） | `PLAN_ROLE_ARN` | `AWS_PROFILE=saigai-readonly-tf terraform output -raw gha_plan_role_arn` |
| 同上 | `TF_STATE_BUCKET` | `terraform output -raw state_bucket` |
| Settings → Environments → `prod` → **Environment secrets** | `DEPLOY_ROLE_ARN` | `terraform output -raw gha_deploy_role_arn` |

## 以後の変更

bootstrap 自体を変えるときは、`saigai-admin` で plan・apply する（CI のデプロイ用ロールは bootstrap のリソースを変えられない）。

```sh
AWS_PROFILE=saigai-readonly-tf terraform plan -out=tfplan   # Claude Code はここまで
AWS_PROFILE=saigai-admin-tf terraform apply tfplan           # 運営者が内容を確認して実行する（先に CLI で saigai-admin のコードを入力）
```
