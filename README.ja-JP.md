# AegisOps

[中文](./README.md) | [日本語](./README.ja-JP.md) | [English](./README.en.md)

AegisOps は、軽量な運用コントロールプレーンを目指した DevOps プラットフォーム MVP です。現在のリポジトリには以下が含まれます。

- Go + Gin + SQLite + GORM + zap によるバックエンド
- React + TypeScript + Vite によるフロントエンドコンソール
- 第一期レビュー、第二期ロードマップ、および中国語ドキュメント

GitHub では中国語の `README.md` を既定表示とし、本ファイルは日本語版の対応ドキュメントです。

## プロジェクト概要

本プロジェクトは、まず第一期 MVP の安全基盤と基本的な運用クローズドループを完成させ、その後第二期として「サービス配布クローズドループ」を構築する方針です。

現在実装済みの主な機能：

- 認証と既定管理者の初期化
- ユーザー、ロール、権限の基本モデル
- Secret 保存とマスク付き返却
- 監査ログの基礎機能
- ホスト CRUD と SSH 接続テスト
- タスク、タスクステップ、タスクログのモデル
- Docker ノード CRUD と基本的なコンテナ操作
- フロントエンドの実ルーティングと基本的な API 連携

## ディレクトリ構成

```text
cmd/         バックエンド起動入口
configs/     設定ファイル
docs/        中国語ドキュメント
frontend/    フロントエンドコンソール
internal/    バックエンド業務実装
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

本番または外部公開環境では必ず変更してください。

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

現在の開発モードでは、フロントエンドは既定で実バックエンドを使用し、`/api` は `http://127.0.0.1:8080` にプロキシされます。

## クイックチェック

ヘルスチェック：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
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

現在のユーザー取得：

```powershell
$token = $login.data.tokens.accessToken
Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/api/auth/me `
  -Headers @{ Authorization = "Bearer $token" }
```

## テスト

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

- [AegisOps 一期 MVP 開発路线](./docs/AegisOps一期MVP开发路线.md)
- [AegisOps 阶段审查报告（一期完成度与二期建议）](./docs/AegisOps阶段审查报告（一期完成度与二期建议）.md)
- [AegisOps 二期前后端开发路线](./docs/AegisOps二期前后端开发路线.md)
- [AegisOps 当前联调阻塞清单](./docs/AegisOps当前联调阻塞清单.md)

## 現在の段階

現在のプロジェクト状況はおおむね以下の通りです。

- 第一期の主幹機能は概ね完成
- 第一期の仕上げは未完了
- 第二期の方向性は明確だが、全面着手前に第一期の収束が推奨される
