# Development Guide

このドキュメントは tfiam に新機能を追加したり、AI アシスタントに開発を依頼するときに参照するガイドです。
「ここを変えたら必ずあそこも変える」「この互換性は絶対に壊してはいけない」という制約を一箇所にまとめています。

---

## CI が自動で守るもの

以下は `go test ./...` が失敗することで壊れたと検知できます。

| 制約 | テスト |
|------|--------|
| `aws.yaml` に 50 件以上のエントリがある | `TestAWSYAMLMinimumEntries` |
| S3・IAM・Lambda・EC2 が必ず存在する | `TestAWSYAMLCoreResourcesPresent` |
| 全アクションが `service:Action` 形式 かつ `high\|best-effort` の confidence | `TestAWSYAMLActionFormat` |
| S3 の create に `s3:CreateBucket` が含まれる | `TestExtract_S3Create` |
| Lambda の create に `iam:PassRole` が含まれる | `TestExtract_LambdaCreate_PassRole` |
| exit code の実装が 0/1/2 を返す | `TestReport_*` |

---

## 手動で守る必要があるもの（同期が必要な箇所）

### ⚠️ 同期ポイント 1: `aws.yaml` の YAML フォーマット

**2 箇所が同じ構造を前提としている。片方を変えたら必ずもう片方も変える。**

| ファイル | 役割 |
|---------|------|
| `cmd/gen-mappings/main.go` の `ActionItem` / `ResourceEntry` / `MappingFile` | yaml を **書く** 側の型 |
| `internal/plan/mapping.go` の `actionEntry` / `resourceMapping` / `mappingFile` | yaml を **読む** 側の型 |

変更が必要なケース：
- 新しいフィールドを追加する（例: `passrole` フラグの追加）
- yaml タグ名を変更する
- ネスト構造を変える

確認方法: `go test ./internal/plan/... -run TestAWSYAMLActionFormat`

---

### ⚠️ 同期ポイント 2: `PermissionSource` interface

`internal/iam/source.go` の interface を変えたら、全ての実装を更新する。

```go
type PermissionSource interface {
    Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error)
    Name() string
}
```

現在の実装:
- `internal/iam/source_file.go` — `FileSource`
- `internal/iam/source_api.go` — `APISource`

interface のシグネチャを変えると **コンパイルエラーになる** ので検知は容易。
ただし `Decision` の意味（`DecisionAllowed=0` など）を変えると振る舞いが壊れるが
コンパイルは通る。`iota` の順番は変えないこと。

---

### ⚠️ 同期ポイント 3: exit code の意味

外部の CI パイプラインが exit code に依存している。**意味を変えてはいけない。**

| Code | 意味 | 変更禁止の理由 |
|------|------|-------------|
| `0` | 問題なし | `|| true` でスキップする CI が壊れる |
| `1` | 権限不足 or 命名違反 | `exit 1` で CI をブロックするパイプラインが壊れる |
| `2` | ツールエラー | エラー監視の閾値として使われる |

定義場所: `internal/report/report.go`

---

### ⚠️ 同期ポイント 4: `perms.yaml` のフォーマット（公開 API）

`--permission-source file` で読み込む `perms.yaml` は外部ユーザーが自分で作るファイルです。
フォーマットを変えると既存ユーザーのファイルが壊れます。

変更する場合はバージョンフィールドを追加して後方互換を保つこと。

定義場所: `internal/iam/source_file.go`

---

## 新機能を追加するときのチェックリスト

### 新しい AWS リソースタイプを追加する

- [ ] `internal/plan/mappings/aws.yaml` にエントリを追加
- [ ] `internal/plan/extract_test.go` にテストを追加
- [ ] Terraform リソース名が自動変換（`AWS::S3::Bucket` → `aws_s3_bucket`）から外れる場合は `cmd/gen-mappings/main.go` の `tfNameOverrides` に追加

### 命名ルールを追加する

- [ ] `internal/naming/rules.go` の `rules` スライスに追加
- [ ] `internal/naming/validator_test.go` にテストを追加

### 新しい権限ソースを追加する（例: GCP 対応）

- [ ] `internal/iam/source.go` の `PermissionSource` interface を実装
- [ ] `internal/cli/check.go` の `buildPermissionSource()` にケースを追加
- [ ] `--permission-source` フラグの説明文を更新

### `aws.yaml` の YAML 構造を変える

- [ ] `cmd/gen-mappings/main.go` の型（`ActionItem` / `ResourceEntry` / `MappingFile`）を変更
- [ ] `internal/plan/mapping.go` の型（`actionEntry` / `resourceMapping` / `mappingFile`）を同じ内容に変更
- [ ] `go test ./internal/plan/... -run TestAWSYAMLActionFormat` でフォーマットを確認

---

## CI/CD ワークフロー一覧

| ファイル | トリガー | 内容 |
|---------|---------|------|
| `.github/workflows/ci.yml` | 全 push / PR | ビルド・vet・テスト・バイナリ動作確認 |
| `.github/workflows/update-mappings.yml` | 毎月1日 / 手動 | CFn スキーマから `aws.yaml` を再生成して PR を作成 |
| `.github/workflows/docs.yml` | `docs/` 変更時 | MkDocs をビルドして GitHub Pages に公開 |

---

## よくある落とし穴

### `go:embed` はビルド時にファイルを焼き込む

`internal/plan/mapping.go` の `//go:embed mappings/aws.yaml` は
**ビルド時点**の `aws.yaml` を埋め込みます。

- テストを実行すると、その時点の `aws.yaml` が使われます
- `gen-mappings` で `aws.yaml` を更新した後にテストを実行すると新しい内容で動きます
- CI では `gen-mappings` 実行 → `go test` の順番が重要

### `gen-mappings` は 50 件以上生成しないと失敗する

API スロットリングなどで全件失敗しても `gen-mappings` は exit 0 を返していました。
現在は 50 件未満のとき `log.Fatalf` で exit 1 します（`cmd/gen-mappings/main.go` 参照）。

### CFn の `DescribeType` はレート制限が厳しい

デフォルト設定（`--concurrency 3`, `--rate-ms 300`）で約 3 req/s。
1615 件の全取得に約 9 分かかります。`update-mappings.yml` のタイムアウトはこれを考慮しています。
