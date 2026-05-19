# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

`AegisOps` は、軽量な運用コントロールプレーン、アプリケーション配信、安定性管理を主軸にした DevOps プラットフォームです。

現在の最新正式版は `v1.0.0` です。単なる雛形ではなく、実際に起動できるバックエンド API、利用可能なフロントエンド管理画面、ローカル向け Demo データ、そして二期計画までそろった一期正式基線になっています。

## プロジェクトの位置づけ

AegisOps は次の用途に向いています。

- 個人用の運用プラットフォーム
- 小規模チーム向け内部運用コンソール
- バックエンド/プラットフォーム系の面接プロジェクト
- DevOps コントロールプレーン製品の原型

現段階では、大規模マルチテナント SaaS よりも、単機または軽量運用基盤としての利用に適しています。

## プロジェクト評価

このプロジェクトの強みは明確です。

- ホスト、Secret、Docker、Registry、サービス配信、Nginx、タスク、アラート、通知、監査を中心にした分かりやすいドメイン設計
- CRUD を超えて、RBAC、監査、配信フロー、ヘルスチェック、エクスポート/バックアップ、スケジューラ API まで備えたバックエンド完成度
- 単なる見た目の殻ではなく、実際の運用オブジェクトを扱えるフロントエンド管理画面
- 製品定位、開発経験、レビュー、二期計画、Release 受け入れ基準までそろったドキュメント資産
- PostgreSQL を主段階データベースとして使いつつ、SQLite もローカル向け軽量モードとして残している導入しやすさ

一方で、現在の境界も正直に見ておくべきです。

- 既定の構成はローカル/単機利用寄り
- エクスポートやバックアップは、フロントエンド画面よりバックエンド/API 側の完成度が高い
- 権限の細粒度化、Secret のライフサイクル管理、高度なスケジューラ、外部通知の本格統合は次段階の重点
- 実ホスト、Docker、Nginx、通知先に対する長期的な実運用検証は、より重い本番利用では引き続き価値がある

要するに、`v1.0.0` 時点でかなり完成度の高い、バックエンド主導の DevOps コントロールプレーン基盤です。

## v1.0.0 で利用できるもの

### バックエンド機能

- 認証・認可
  - 管理者初期化
  - ログイン認証
  - ユーザー/ロール管理
  - RBAC
  - 監査ログ
- 資産・認証情報
  - ホスト管理
  - SSH 接続テスト
  - Secret の暗号化保存とマスク返却
- 運用実行
  - タスクセンター
  - タスクステップとログ
  - Web ターミナル
- 配信フロー
  - Docker ノード管理
  - イメージ Registry 管理
  - サービス定義
  - リリース、アップグレード、ロールバック
  - リリース後のヘルスチェックとロールバック提案
- 安定性管理
  - 通知チャネル
  - アラートルール
  - アラートイベント
  - ホスト/サービスのヘルスチェック
  - Nginx ノードと設定の配信/ロールバック
- プラットフォーム支援
  - エクスポート API
  - バックアップ API
  - スケジューラ API
  - Demo データ投入
  - PostgreSQL 主段階対応、SQLite は軽量オプション

### 実装済みフロントエンド画面

- ダッシュボード
- 資産管理
  - ホスト
  - Secret
- 実行リソース
  - Docker ノード
  - Nginx ノード
- アプリ配信
  - Registry
  - サービス定義
- タスク一覧とタスク詳細
- アラートイベント
- 通知チャネル
- アラートルール
- 監査ログ
- システム管理
  - ユーザー
  - ロール
  - スケジュールタスク
- ログイン
- 管理者初期化
- Web ターミナル

補足：

- 開発時のフロントエンドは既定で実バックエンドに接続します
- エクスポートとバックアップは、現状ではフロント画面よりバックエンド/API 側が先行しています

## ディレクトリ構成

```text
cmd/         バックエンド起動入口
configs/     設定ファイル
data/        ローカル実行生成物と任意の SQLite ファイル
deploy/      デプロイ/スモークテスト用リソース
docs/        中国語のプロジェクト文書と計画
frontend/    フロントエンド管理画面
internal/    バックエンド業務実装
logs/        ローカル実行ログ
pkg/         共通パッケージ
scripts/     ローカル補助スクリプト
```

## バックエンド起動

必要環境：

- Go 1.24+
- Windows / Linux / macOS

起動コマンド：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go run ./cmd/aegisops
```

既定設定ファイル：

- [configs/config.yaml](./configs/config.yaml)

既定リッスンアドレス：

- `:8080`

既定データベース：

- PostgreSQL: `postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable`
- SQLite の任意サンプル設定: `configs/config.sqlite.example.yaml`

既定管理者：

```text
username: admin
password: admin123456
```

これらの既定値はローカル開発とデモ専用です。継続利用前に管理者パスワード、JWT Secret、Secret Key、PostgreSQL パスワードを必ず変更してください。

## フロントエンド起動

必要環境：

- Node.js 18+
- npm 9+

起動コマンド：

```powershell
Set-Location frontend
npm install
npm run dev
```

既定 URL：

- [http://localhost:4173](http://localhost:4173)

開発モードでは、フロントエンドは既定で実バックエンドを利用し、`/api` を `http://127.0.0.1:8080` へプロキシします。

本番相当のプレビュー：

```powershell
Set-Location frontend
npm run build
npm run preview
```

## スモークチェック

ヘルスチェック：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
Invoke-RestMethod http://127.0.0.1:8080/readyz
```

ログイン例：

```powershell
$body = @{ username = "admin"; password = "admin123456" } | ConvertTo-Json
$login = Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8080/api/auth/login `
  -ContentType "application/json" `
  -Body $body
$login.data.tokens.accessToken
```

現在ユーザー取得：

```powershell
$token = $login.data.tokens.accessToken
Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/api/auth/me `
  -Headers @{ Authorization = "Bearer $token" }
```

## テストと Release 基線

バックエンドテスト：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

フロントエンドのビルド確認：

```powershell
Set-Location frontend
npm run build
```

この 2 つを最低限の事前確認として扱うことを推奨します。

正式 Release の受け入れは、以下を参照してください。

- [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)

## 環境変数による上書き

`AEGISOPS_` プレフィックス付き環境変数でバックエンド設定を上書きできます。

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_DATABASE_DRIVER = "postgres"
$env:AEGISOPS_DATABASE_DSN = "postgres://aegisops:replace-me@127.0.0.1:5432/aegisops?sslmode=disable"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

ローカル PostgreSQL のクイック起動:

```powershell
docker compose -f deploy/postgres/docker-compose.yaml up -d
```

## ドキュメント案内

主な文書：

- [AegisOps产品定位与目标用户分析](./docs/AegisOps产品定位与目标用户分析.md)
- [AegisOps产品未来演进方向分析](./docs/AegisOps产品未来演进方向分析.md)
- [AegisOps一期MVP开发路线](./docs/AegisOps一期MVP开发路线.md)
- [AegisOps一期开发手册（真实经验与排查方法）](./docs/AegisOps一期开发手册（真实经验与排查方法）.md)
- [AegisOps阶段审查报告（一期完成度与二期建议）](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps UIUX视觉与易用性专业优化方案](./docs/AegisOps%20UIUX视觉与易用性专业优化方案.md)
- [AegisOps二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps二期专项规划：通知告警与健康检查闭环](./docs/AegisOps二期专项规划：通知告警与健康检查闭环.md)
- [AegisOps二期专项规划：导出、备份与故障排查包](./docs/AegisOps二期专项规划：导出、备份与故障排查包.md)
- [AegisOps二期专项规划：权限细粒度、密钥管理与任务调度](./docs/AegisOps二期专项规划：权限细粒度、密钥管理与任务调度.md)
- [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md)

## 現在の段階

このリポジトリは現在、次のように整理できます。

- `v1.0.0` の正式版基線を完成済み
- ローカル実行、デモ、API 連携、二期開発の基礎として十分利用可能
- 今後は一期主幹構築よりも、本番強化と二期重点能力の拡張が中心
