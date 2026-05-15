# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

AegisOps は、軽量な運用コントロールプレーン、デリバリー、安定性管理に焦点を当てた DevOps プラットフォームです。現在のリポジトリには以下が含まれます。

- Go + Gin + SQLite + GORM + zap によるバックエンド API とタスク実行基盤
- React + TypeScript + Vite によるフロントエンドコンソール
- 中国語を優先した製品、開発、レビュー、第二期計画、Release 文書

GitHub では中国語の `README.md` を既定表示とし、本ファイルは日本語版の対応ドキュメントです。

## プロジェクト概要

現在の実装状況に基づくと、AegisOps はローカルで実行可能な運用プラットフォームの土台をすでに備えています。

- 認証と権限: 管理者初期化、ログイン認証、ユーザー、ロール、RBAC、監査ログ
- 資産とシークレット: ホスト管理、SSH 接続テスト、暗号化 Secret 保存、マスク付き返却
- 運用実行: タスク、タスクステップ、タスクログ、Web ターミナル
- コンテナとデリバリー: Docker ノード、イメージ Registry、サービス定義、リリース、アップグレード、ロールバック
- 安定性管理: 通知チャネル、アラートルール、アラートイベント、ホスト/サービスのヘルスチェック
- リリース閉ループ: リリース後の自動疎通確認、ロールバック提案、Nginx ノード管理と設定ロールバック
- プラットフォーム支援: エクスポート、バックアップ、スケジューラ API、開発環境向け Demo データ投入

フロントエンドでは、ダッシュボード、ホスト、Secret、Docker、Nginx、Registry、サービス、タスク、監査、アラート、通知、ユーザー、ロール、ターミナル、ログイン、管理者初期化などの主要ページが利用できます。

## ディレクトリ構成

```text
cmd/         バックエンド起動入口
configs/     設定ファイル
data/        SQLite データとローカル実行生成物
docs/        中国語のプロジェクト文書と計画
frontend/    フロントエンドコンソール
internal/    バックエンド業務実装
logs/        ローカル実行ログ
pkg/         共通パッケージ
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

既定 SQLite データベース：

- `data/aegisops.db`

既定管理者：

```text
username: admin
password: admin123456
```

ローカル開発以外で利用する前に、既定管理者パスワード、JWT Secret、Secret Key を必ず変更してください。

`app.env` が `dev`、`development`、`test` の場合、バックエンドは Demo Registry、Demo Docker ノード、サンプルサービス、インスタンスデータを自動投入し、リリース、ロールバック、ヘルスチェックのローカル検証をしやすくします。

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

本番相当のプレビューを行う場合：

```powershell
Set-Location frontend
npm run build
npm run preview
```

## クイックチェック

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

## テストと受け入れ

バックエンドテスト：

```powershell
$env:GOCACHE = "$PWD\.gocache"
go test ./...
```

フロントエンドビルド確認：

```powershell
Set-Location frontend
npm run build
```

この 2 つは各コミット前の最低限の確認として推奨されます。正式 Release の受け入れには [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md) を参照してください。

## 環境変数による上書き

`AEGISOPS_` プレフィックス付き環境変数でバックエンド設定を上書きできます。

```powershell
$env:AEGISOPS_HTTP_ADDR = ":18080"
$env:AEGISOPS_DATABASE_DSN = "data/dev.db"
$env:AEGISOPS_SECURITY_JWT_SECRET = "replace-me"
$env:AEGISOPS_ADMIN_PASSWORD = "replace-me-too"
```

## ドキュメント案内

主要ドキュメント：

- [AegisOps 一期 MVP 开发路线](./docs/AegisOps一期MVP开发路线.md)
- [AegisOps 一期开发手册（真实经验与排查方法）](./docs/AegisOps一期开发手册（真实经验与排查方法）.md)
- [AegisOps 阶段审查报告（一期完成度与二期建议）](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps 正式 Release 验收清单](./docs/AegisOps正式Release验收清单.md)
- [AegisOps 二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps 二期专项规划：通知告警与健康检查闭环](./docs/AegisOps二期专项规划：通知告警与健康检查闭环.md)
- [AegisOps 二期专项规划：导出、备份与故障排查包](./docs/AegisOps二期专项规划：导出、备份与故障排查包.md)
- [AegisOps 二期专项规划：权限细粒度、密钥管理与任务调度](./docs/AegisOps二期专项规划：权限细粒度、密钥管理与任务调度.md)

## 現在の段階

このリポジトリは、もはや単なるスキャフォールドではありません。バックエンド API、フロントエンド業務ページ、Demo データ、基本的な結合ループがすでに揃っているため、ローカル開発、デモ、API 連携を直接進められます。

ただし、現時点では次のように捉えるのが適切です。

- 第一期の主幹機能は概ね完成
- 第一期の収束、運用向け受け入れ確認、いくつかの重点強化は継続中
- 正式 Release 可否は [AegisOps正式Release验收清单](./docs/AegisOps正式Release验收清单.md) を基準に判断するのが適切
