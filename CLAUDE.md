# tfiam — Development Constraints for AI Assistants

このファイルは Claude などの AI アシスタントが新しいセッションで開発を引き継ぐときに
読むべき制約・同期ポイント・禁止事項をまとめたものです。
人間の開発者も同様に参照してください。詳細は `docs/development.md` にあります。

---

## 絶対に壊してはいけないもの

### 1. exit code の意味（外部 CI パイプラインが依存）

| Code | 意味 |
|------|------|
| `0` | 問題なし |
| `1` | 権限不足 or 命名違反 |
| `2` | ツールエラー |

定義: `internal/report/report.go` — iota の順番も変えないこと。

### 2. `perms.yaml` のフォーマット（ユーザーが手書きするファイル）

変更する場合はバージョンフィールドを追加して後方互換を保つ。
定義: `internal/iam/source_file.go`

### 3. `PermissionSource` interface のシグネチャ

```go
// internal/iam/source.go
type PermissionSource interface {
    Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error)
    Name() string
}
```

シグネチャを変えたら `FileSource`（source_file.go）と `APISource`（source_api.go）の両方を更新する。

---

## 変更時に必ず 2 箇所同時に変えるもの

### `aws.yaml` の YAML 構造

| ファイル | 型名 |
|---------|------|
| `cmd/gen-mappings/main.go` | `ActionItem`, `ResourceEntry`, `MappingFile` |
| `internal/plan/mapping.go` | `actionEntry`, `resourceMapping`, `mappingFile` |

yaml タグ名・フィールド名・ネスト構造のどれかを変えたら**必ず両方**変える。
確認: `go test ./internal/plan/... -run TestAWSYAMLActionFormat`

---

## CI が守っていること（`go test ./...` で検知）

- `aws.yaml` に 50 件以上のリソースエントリがある（`TestAWSYAMLMinimumEntries`）
- S3・IAM・Lambda・EC2 が必ず存在する（`TestAWSYAMLCoreResourcesPresent`）
- 全アクションが `service:Action` 形式（`TestAWSYAMLActionFormat`）
- extract ロジックが正しいアクションを返す（`TestExtract_*`）

---

## 重要な実装上の注意

- `//go:embed mappings/aws.yaml` はビルド時にファイルを焼き込む。
  `gen-mappings` 実行後に `go test` すると新しい内容が使われる。
- `gen-mappings` は 50 件未満しか生成できなかった場合 exit 1 で終了する（空ファイル上書き防止）。
- CFn `DescribeType` のデフォルト設定は `--concurrency 3 --rate-ms 300`（約 3 req/s）。
  並列数を上げると全件スロットリングされる。

---

## 詳細ドキュメント

- 同期ポイントの詳細: `docs/development.md`
- アーキテクチャ: `docs/how-it-works.md`
- コードツアー: `TOUR.md`
