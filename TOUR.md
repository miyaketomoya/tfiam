# tfiam コードツアー

このドキュメントは「コードを読む順番」と「その箇所で学べる Go の概念」をセットで案内します。
`go test ./...` でテストを回しながら読むと理解が深まります。

---

## ローカル実行（AWS 不要）

```bash
# ビルド
go build -o tfiam ./cmd/tfiam

# plan.json を用意（AWS 不要の .json パス）
terraform show -json plan.tfplan > plan.json
# または testdata/plans/s3_create.json を使う

# file モード（AWS コールなし）で実行
./tfiam check testdata/plans/s3_create.json \
  --principal arn:aws:iam::123456789012:user/you \
  --permission-source file \
  --permissions-file testdata/permissions/s3_denied.yaml

# テストだけ走らせる
go test ./...
```

---

## 読む順番マップ

```
go.mod                         ← 1. モジュールと依存関係
cmd/tfiam/main.go              ← 2. エントリポイント
internal/cli/root.go           ← 3. CLI フラグ定義
internal/cli/check.go          ← 4. 全体の処理フロー（地図）
  │
  ├─ internal/plan/loader.go   ← 5. ファイル読み込み
  ├─ internal/plan/mapping.go  ← 6. go:embed・YAML 読み込み
  ├─ internal/plan/extract.go  ← 7. Plan 解析・重複除去
  │
  ├─ internal/iam/source.go    ← 8. interface の定義
  ├─ internal/iam/source_file.go ← 9. interface の実装（ファイル版）
  ├─ internal/iam/source_api.go  ← 10. interface の実装（AWS 版）+ 依存注入
  ├─ internal/iam/checker.go   ← 11. interface を使う側
  │
  ├─ internal/naming/rules.go  ← 12. 構造体テーブル
  ├─ internal/naming/validator.go ← 13. map[string]interface{} の扱い
  │
  └─ internal/report/report.go ← 14. io.Writer・exit code 契約
```

---

## Stop 1 — `go.mod`  ▶ Go モジュールの仕組み

```
module github.com/tfiam-dev/tfiam
go 1.22.1
```

- `module` 行がこのリポジトリの「住所」。import パスはここから始まる。
- `require` ブロックが依存パッケージのバージョン固定。
- `go.sum` は改ざん検知用のハッシュ台帳。

**やってみる**: `go mod graph` で依存ツリーを見る。

---

## Stop 2 — `cmd/tfiam/main.go`  ▶ エントリポイント・型アサーション

```go
func main() {
    cmd := cli.NewRootCmd()
    cmd.SilenceErrors = true
    if err := cmd.Execute(); err != nil {
        if e, ok := err.(cli.ExitError); ok {   // ← 型アサーション
            os.Exit(e.ExitCode())
        }
        os.Exit(1)
    }
}
```

**Go 概念**:
- `e, ok := err.(T)` — インターフェース値から具体型を取り出す（型アサーション）。`ok` が false なら変換失敗。
- `cmd/` は「実行ファイルを作るパッケージ」の慣習的な置き場。`internal/` は外部から import できないパッケージ。

---

## Stop 3 — `internal/cli/root.go`  ▶ cobra・フラグ定義

```go
func NewRootCmd() *cobra.Command {
    root := &cobra.Command{Use: "tfiam", ...}
    root.PersistentFlags().StringVar(&gf.Principal, "principal", "", "...")
    ...
}
```

**Go 概念**:
- **cobra**: CLI フレームワーク。`Command` 構造体にサブコマンドを `AddCommand()` で追加する。
- `PersistentFlags()` はサブコマンドにも引き継がれるフラグ。`Flags()` はそのコマンド専用。
- `StringVar(&変数, "フラグ名", "デフォルト", "説明")` — フラグの値を既存の変数に直接書き込む。

---

## Stop 4 — `internal/cli/check.go`  ▶ 全体の処理フロー（地図として読む）

ここは「何をどの順番で呼ぶか」だけを読む。実装詳細は後の Stop で追う。

```go
func runCheck(ctx context.Context, gf *GlobalFlags, path string) error {
    plan   := loader.Load(path)        // Stop 5
    req    := extractor.Extract(plan)  // Stop 7
    naming := validator.Validate(plan) // Stop 13
    source := buildPermissionSource()  // Stop 9/10
    perms  := checker.Check(req)       // Stop 11
    report.Render(perms, naming)       // Stop 14
}
```

**Go 概念**:
- `context.Context` — タイムアウトやキャンセルを伝搬する仕組み。関数の第一引数に渡すのが慣習。
- エラーハンドリングは `if err != nil { return err }` を繰り返す。例外はない。

---

## Stop 5 — `internal/plan/loader.go`  ▶ ファイル IO・サブプロセス

```go
case ".json":
    data, _ := os.ReadFile(path)
    json.Unmarshal(data, &p)          // AWS コールなし

case ".tfplan":
    cmd := exec.CommandContext(ctx, "terraform", "show", "-json", path)
    out, _ := cmd.Output()            // サブプロセス起動
    json.Unmarshal(out, &p)
```

**Go 概念**:
- `os.ReadFile` でバイト列を取得し `json.Unmarshal` で構造体に変換する 2 ステップが基本パターン。
- `exec.CommandContext` でシェルコマンドをプロセスとして呼び出す。`Output()` は stdout を返す。
- `filepath.Ext(path)` でファイル拡張子を取得。`switch` で分岐。

---

## Stop 6 — `internal/plan/mapping.go`  ▶ `go:embed`・YAML 読み込み

```go
//go:embed mappings/aws.yaml     // ← ビルド時にファイルを埋め込む
var awsYAML []byte

func init() {
    yaml.Unmarshal(awsYAML, &registry)
}
```

**Go 概念**:
- `//go:embed` はコンパイル時にファイルをバイナリに焼き込むディレクティブ。配布が楽になる。
- `init()` はパッケージ初期化時に自動で呼ばれる関数（`main` より前に実行される）。
- `mappings/aws.yaml` の中身を `internal/plan/mappings/aws.yaml` にコピーして `go:embed` に一致させた理由: `go:embed` は `../ ` パスを使えない制約がある。

---

## Stop 7 — `internal/plan/extract.go`  ▶ スライス・マップ・重複除去

```go
type RequiredAction struct {
    ResourceType    string
    ResourceAddress string
    Action          string
    ResourceArn     string
    Confidence      string
}

seen := map[string]bool{}
key  := action + "\x00" + resourceArn
if seen[key] { continue }
seen[key] = true
result = append(result, ra)
```

**Go 概念**:
- `map[string]bool` を `set` として使うパターン（Go に Set 型はない）。
- `append(slice, elem)` でスライスに追加。返り値を受け取り直す必要がある。
- `\x00`（null バイト）をセパレータにすることでキーの衝突を防ぐテクニック。

---

## Stop 8 — `internal/iam/source.go`  ▶ interface の定義

```go
type Decision int
const (
    DecisionAllowed Decision = iota
    DecisionDenied
    DecisionUnknown
)

type PermissionSource interface {
    Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error)
    Name() string
}
```

**Go 概念**:
- **interface** は「このメソッドを持っていれば OK」という契約。継承は不要。
- `iota` は 0 から始まる連番定数。`DecisionAllowed=0, DecisionDenied=1, DecisionUnknown=2`。
- `Decision int` のように `type 名前 基底型` で独自型を作ると、間違った int を渡すことをコンパイラが防ぎやすくなる。

---

## Stop 9 — `internal/iam/source_file.go`  ▶ interface の実装・2 段階フォールバック

```go
type FileSource struct {
    index         map[string]Decision  // "action\x00resource" → decision
    wildcardIndex map[string]Decision  // "action\x00*"        → decision
}

func (fs *FileSource) Evaluate(ctx context.Context, p Principal, action, resourceArn string) (Decision, error) {
    key := action + "\x00" + resourceArn
    if d, ok := fs.index[key]; ok { return d, nil }         // 完全一致
    if d, ok := fs.wildcardIndex[action]; ok { return d, nil } // * フォールバック
    return DecisionUnknown, nil
}
```

**Go 概念**:
- Go の interface 実装は **暗黙的**。`implements PermissionSource` と書く必要がない。メソッドが揃っていれば自動で実装扱いになる。
- レシーバ `(fs *FileSource)` の `*` はポインタレシーバ。フィールドを変更する場合や、大きな構造体をコピーしたくない場合はポインタを使う。

---

## Stop 10 — `internal/iam/source_api.go`  ▶ 依存注入・テスト可能な設計

```go
// テスト用に差し替えられる interface
type IAMSimulatorClient interface {
    SimulatePrincipalPolicy(ctx context.Context, in *iam.SimulatePrincipalPolicyInput, ...) (*iam.SimulatePrincipalPolicyOutput, error)
}

type APISource struct {
    client IAMSimulatorClient   // ← 本物の AWS クライアントでもモックでも OK
}

// 本番用
func NewAPISource(cfg aws.Config) *APISource { ... }

// テスト用（モックを注入）
func NewAPISourceWithClient(client IAMSimulatorClient) *APISource { ... }
```

**Go 概念**:
- **依存注入 (Dependency Injection)**: 実装をハードコードせず、interface 型で受け取る。テスト時は偽物 (stub/mock) を差し込める。
- `source_api_test.go` に `stubIAMClient` という構造体があり、これが「テスト用の偽 AWS」として動いている。

---

## Stop 11 — `internal/iam/checker.go`  ▶ interface を使う側

```go
type Checker struct {
    source PermissionSource  // FileSource でも APISource でも受け取れる
}

func (c *Checker) Check(ctx context.Context, principal iam.Principal, required []plan.RequiredAction) ([]report.PermFinding, error) {
    for _, ra := range required {
        decision, err := c.source.Evaluate(ctx, principal, ra.Action, ra.ResourceArn)
        ...
    }
}
```

**Go 概念**:
- `Checker` は `PermissionSource` interface に依存しているだけで、FileSource や APISource を知らない。
- これが **依存逆転の原則**: 上位モジュール（Checker）が下位実装（FileSource/APISource）に依存するのでなく、両者が interface に依存する。

---

## Stop 12 — `internal/naming/rules.go`  ▶ 構造体スライス・テーブル駆動

```go
type Rule struct {
    ResourceType string
    NameField    string
    MinLength    int
    MaxLength    int
    Pattern      *regexp.Regexp
}

var rules = []Rule{
    {"aws_s3_bucket", "bucket", 3, 63, regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)},
    {"aws_iam_role",  "name",   1, 64, nil},
    ...
}
```

**Go 概念**:
- **テーブル駆動**: ロジックをデータとして表現する Go の定番パターン。`if/else` の連鎖を避けられる。
- `regexp.MustCompile` はコンパイルに失敗したらパニックする版。グローバル変数の初期化に使うのが慣習（起動時に即クラッシュさせる）。
- `*regexp.Regexp` の `*` はポインタ型。nil で「パターンなし」を表現している。

---

## Stop 13 — `internal/naming/validator.go`  ▶ `map[string]interface{}` の扱い

```go
// terraform-json は After フィールドを interface{} で返す
afterMap, ok := change.After.(map[string]interface{})
value, exists := afterMap[rule.NameField]

// after_unknown は bool として入ってくる
unknownMap, _ := change.AfterUnknown.(map[string]interface{})
if unknownMap[rule.NameField] == true {
    // apply 後にしか名前が決まらない → スキップ
}
```

**Go 概念**:
- JSON を `interface{}` で受け取ると実際の型は `map[string]interface{}` や `[]interface{}` になる。型アサーションで具体型を取り出す必要がある。
- `value.(string)` のように 2 段階の型アサーションが必要なことが多い。

---

## Stop 14 — `internal/report/report.go`  ▶ `io.Writer`・exit code 設計

```go
func (r *Report) Render(w io.Writer, format string) int {
    ...
    enc := json.NewEncoder(w)
    enc.Encode(out)
    return exitCode  // 0, 1, 2
}
```

**Go 概念**:
- `io.Writer` は `Write([]byte) (int, error)` を持つ interface。stdout でも bytes.Buffer でも渡せるので**テストが書きやすい**。
- exit code をアプリ側で決めて `os.Exit()` で終了するのが CLI の慣習。`os.Exit` はデファードも実行しないので呼び出しは `main` だけに限定する。
- `report_test.go` では `bytes.Buffer` を `w` に渡して出力文字列を検証している。

---

## Go 概念チートシート（このプロジェクトで登場するもの）

| 概念 | ファイル | 一言メモ |
|------|---------|---------|
| モジュール | `go.mod` | `go get` で依存を追加 |
| 型アサーション | `main.go` | `v, ok := x.(T)` |
| interface | `iam/source.go` | メソッドが揃えば自動実装 |
| iota | `iam/source.go` | 連番定数 |
| ポインタレシーバ | `iam/source_file.go` | `(fs *FileSource)` |
| 依存注入 | `iam/source_api.go` | interface 経由で差し替え可能に |
| go:embed | `plan/mapping.go` | ファイルをバイナリに埋め込み |
| テーブル駆動テスト | `naming/rules.go` | データでロジックを表現 |
| `io.Writer` | `report/report.go` | 出力先を抽象化 |
| `context.Context` | 全体 | タイムアウト・キャンセル伝搬 |
| `map[K]V` を set に | `plan/extract.go` | `map[string]bool` で重複排除 |

---

## テストを読む順番

テストはコードの仕様書でもあります。実装を読んだ後に対応するテストを読むと理解が深まります。

```
internal/plan/extract_test.go      ← TestExtract_*
internal/iam/source_file_test.go   ← TestFileSource_*
internal/iam/source_api_test.go    ← TestAPISource_* （stub の使い方）
internal/iam/checker_test.go       ← TestChecker_*
internal/naming/validator_test.go  ← TestValidator_*
internal/report/report_test.go     ← TestReport_* （io.Writer の使い方）
```
