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
| 運営者の IAM ユーザーのポリシー | 上の2つのロールの引き受けと、自分のパスワード・MFA の管理だけ。`aws login` 用に AWS 管理ポリシー `SignInLocalDevelopmentAccess` も付ける |

## 前提

- AWS CLI 2.3x 以上（`aws login` が使えること）
- Terraform 1.16.5（`infra/.terraform-version`。tfenv なら自動で使われる）
- 運営者用の IAM ユーザーをコンソールで作成済み（パスワード＋MFA。**アクセスキーは作らない**）。名前は本名と結び付かないもの（例: `saigai-operator`）にする（ARN やログに出るため）
- ルートユーザー（アカウントを作ったメールアドレス）で、次を済ませておく。ルートユーザーは、これ以後は使わない
  - MFA（パスキーか認証アプリ）を登録する: 右上のアカウント名 →「セキュリティ認証情報」→「多要素認証（MFA）」
  - IAM ユーザーに請求情報の閲覧を許可する（手順 7 の予算アラームに必要）: 右上のアカウント名 →「アカウント」→「IAM ユーザーおよびロールによる請求情報へのアクセス」→「IAM アクセスをアクティブ化」。ルートユーザーにしかできず、これがないと管理者権限があっても予算を作れない
  - （任意）アカウントのエイリアスを、本名と結び付かない名前で作る（サインインの URL に出るため）

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

1. コンソールで、IAM ユーザーから `AdministratorAccess` を外す（`saigai-operator` のインラインポリシーと `SignInLocalDevelopmentAccess` だけになる）。
   `SignInLocalDevelopmentAccess` が付いていないと、`aws login` のログイン状態を更新できなくなる（`signin:CreateOAuth2Token` がないというエラー）
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
| 同上 | `TF_VARS_PROD` | `infra/envs/prod/terraform.tfvars` の中身をそのまま（アカウント ID・ドメイン・メールアドレスを含むため Secrets）。手元の tfvars を変えたら、こちらも同じ内容に更新する |
| Settings → Environments → `prod` → **Environment secrets** | `DEPLOY_ROLE_ARN` | `terraform output -raw gha_deploy_role_arn` |

Environments は2つ作る（`deploy-prod.yml` が使う。docs/06 5.2）。

| Environment | 設定 |
| --- | --- |
| `prod-plan` | Deployment branches and tags → **Selected branches and tags** で、タグ `v*` だけを許す。承認は不要（plan は読み取り専用） |
| `prod` | **Required reviewers** に自分を入れる。Deployment branches and tags はタグ `v*` だけ。「Prevent self-review」は外す（個人運用のため、自分で承認する） |

リリースは `git tag v0.1.0 && git push origin v0.1.0` で始まる。タグを作れる人を絞るため、Settings → Rules → Rulesets でタグ `v*` の作成・削除を自分だけに制限する。

### 7. 予算アラームを作る

月 $50 の予算に、実績 $20（40%）・実績 $50（100%）・予測 $50 超えの3つの通知を付ける（docs/05 4章）。
通知先のメールアドレスを公開リポジトリに載せないため、Terraform では管理せず、CLI で1回だけ作る（名前は `saigai-monthly`）。
`saigai-admin` のロールで実行する。

```sh
export AWS_PROFILE=saigai-admin
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
printf '通知を受け取るメールアドレス: '; read -r EMAIL   # bash・zsh のどちらでも動く書き方
sub='[{"SubscriptionType":"EMAIL","Address":"'"$EMAIL"'"}]'
aws budgets create-budget --account-id "$ACCOUNT_ID" \
  --budget '{"BudgetName":"saigai-monthly","BudgetLimit":{"Amount":"50","Unit":"USD"},"TimeUnit":"MONTHLY","BudgetType":"COST"}' \
  --notifications-with-subscribers '[
    {"Notification":{"NotificationType":"ACTUAL","ComparisonOperator":"GREATER_THAN","Threshold":40,"ThresholdType":"PERCENTAGE"},"Subscribers":'"$sub"'},
    {"Notification":{"NotificationType":"ACTUAL","ComparisonOperator":"GREATER_THAN","Threshold":100,"ThresholdType":"PERCENTAGE"},"Subscribers":'"$sub"'},
    {"Notification":{"NotificationType":"FORECASTED","ComparisonOperator":"GREATER_THAN","Threshold":100,"ThresholdType":"PERCENTAGE"},"Subscribers":'"$sub"'}
  ]'
aws budgets describe-budget --account-id "$ACCOUNT_ID" --budget-name saigai-monthly --query 'Budget.BudgetLimit'
unset EMAIL sub
```

## 以後の変更

bootstrap 自体を変えるときは、`saigai-admin` で plan・apply する（CI のデプロイ用ロールは bootstrap のリソースを変えられない）。

```sh
AWS_PROFILE=saigai-readonly-tf terraform plan -out=tfplan   # Claude Code はここまで
AWS_PROFILE=saigai-admin-tf terraform apply tfplan           # 運営者が内容を確認して実行する（先に CLI で saigai-admin のコードを入力）
```

## うまくいかないとき

| 症状 | 確認すること |
| --- | --- |
| `aws login` がない（`Invalid choice`） | AWS CLI を更新する（`aws --version` が 2.3x 以上か） |
| `aws sts get-caller-identity` が `root` になる | ブラウザでルートユーザーのままサインインしていないか。サインアウトして IAM ユーザーでログインし直す |
| `Your session has expired. Please reauthenticate using 'aws login'` | IAM ユーザーのログインが切れている。`aws login --profile saigai` のあとで、ロールのプロファイルで MFA のコードを入力し直す |
| Terraform が `Token has expired` などで止まる | ロールのセッション（最長 1 時間）が切れている。`aws sts get-caller-identity --profile saigai-readonly`（または `saigai-admin`）で MFA のコードを入力し直す |
| `terraform validate` で、既存の `error_message` が「英語の文」でないと言われる | 古い Terraform（1.0 系など）で動いている。tfenv は実行したディレクトリから上にたどって `.terraform-version` を探すため、リポジトリのルートから `-chdir` で実行すると見つからない。`infra/` の中で実行する |
| 予算の作成で `AccessDeniedException` | 前提の「請求情報の閲覧の許可」が済んでいるか |
| `terraform plan` で `allowed_account_ids` のエラー | `terraform.tfvars` の `account_id` と、ログインしているアカウントが一致しているか |
| GitHub Actions で `Not authorized to perform sts:AssumeRoleWithWebIdentity` | トークンの `sub` が信頼条件と一致しているか。CloudTrail の `AssumeRoleWithWebIdentity` の拒否の記録で、届いた `sub` を確認できる（このリポジトリは `repo:<owner>@<id>/<name>@<id>:...` の形式。docs/06 4.1） |
| デプロイが `not authorized to perform: <操作>` で失敗する | デプロイ用ロールに許可するサービスが足りない。`policies.tf` の `deploy_services` に足して、この bootstrap を apply する |
