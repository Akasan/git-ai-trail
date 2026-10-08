# git-ai-trail

Gitで生成されたAIコードの変更を追跡し、AI支援の監査証跡を維持します。

[English README](README.md)

## 動機

AI時代において、以下のことがますます重要になっています:
- **出所の追跡**: AIと人間のどちらがコードを書いたか把握する
- **理解の確保**: 開発者がマージするAI生成コードを理解していることを確認する
- **監査証跡**: コンプライアンス、セキュリティレビュー、コード品質のために記録を保持する

`git-ai-trail`は、チームがAI生成コードの監査証跡を保持するのを支援し、作成者がAI作成の変更を理解していることを証明するまでマージをブロックできる将来のツールを可能にします。

## インストール

```bash
go install github.com/Akasan/git-ai-trail@latest
```

またはソースからビルド:

```bash
git clone https://github.com/Akasan/git-ai-trail
cd git-ai-trail
go build -o git-ai-trail
sudo mv git-ai-trail /usr/local/bin/
```

## クイックスタート

1. **リポジトリを初期化**:
   ```bash
   git ai-trail init
   ```

2. **AI生成の変更をマーク**:
   ```bash
   # AIがファイルを編集した後、スナップショットを取る
   git ai-trail mark --model gpt-4 --agent cursor
   ```

3. **帰属情報付きでコミット**:
   ```bash
   git ai-trail commit -m "AIの支援で機能を追加"
   ```

4. **AI帰属情報を表示**:
   ```bash
   # 未コミットの変更を確認
   git ai-trail status
   
   # 帰属履歴を表示
   git ai-trail log
   
   # AIコンテキスト付きblame
   git ai-trail blame src/main.go
   ```

## コマンド

### `git ai-trail mark [options] [paths...]`

現在のファイル変更をAI生成として記録します。AIがファイルを編集した直後に呼び出してください。

**オプション:**
- `--model NAME`: AIモデル名（例: `gpt-4`, `claude-3`）
- `--agent NAME`: AIエージェント/エディタ名（例: `cursor`, `github-copilot`）
- `--prompt TEXT`: プロンプトテキスト（200文字に切り詰められます）
- `--prompt-file PATH`: ファイルからプロンプトを読み込む
- `--stdin-json`: stdinからJSONでファイルパスを読み取る（`tool_input.file_path`を提供するエディタフック用）
- `--quiet`, `-q`: 出力を抑制

**例:**
```bash
# すべての変更されたファイルをマーク
git ai-trail mark --model gpt-4 --agent cursor

# 特定のファイルをマーク
git ai-trail mark src/main.go src/util.go

# プロンプトコンテキストを含める
git ai-trail mark --prompt "OAuthログインを実装" --model gpt-4
```

### `git ai-trail status`

未コミットの変更のAI帰属情報を表示します。

```bash
$ git ai-trail status
未コミット変更のAI帰属情報:

src/main.go:
  45 ai, 3 ai-modified, 12 human (80.0% AI)

全体: 45 ai, 3 ai-modified, 12 human (80.0% AI)
```

### `git ai-trail commit [git commit args...]`

変更をコミットし、git notesにAI帰属情報を自動的に記録します。

```bash
git ai-trail commit -m "新機能を追加"
```

### `git ai-trail record [<commit>]`

既存のコミットに帰属記録を追加します。post-commitフックで役立ちます。

```bash
# HEADの帰属を記録
git ai-trail record

# 特定のコミットの記録
git ai-trail record abc123
```

### `git ai-trail install-hooks`

post-commitフックをインストールして、帰属を自動的に記録します。

```bash
git ai-trail install-hooks
```

インストール後は、コミット前に`git ai-trail mark`を実行するだけで済みます。帰属は自動的に記録されます。

### `git ai-trail blame <file>`

行ごとのAI帰属情報を表示します（`git blame`のように）。

```bash
$ git ai-trail blame src/main.go
a1b2c3d AI    1) package main
a1b2c3d AI    2) 
a1b2c3d AI*   3) func main() {
a1b2c3d HUM   4)     println("hello")
```

ラベル:
- `AI`: AI生成、変更なし
- `AI*`: AI生成、その後人間が修正
- `HUM`: 人間が作成

### `git ai-trail log [git log args...]`

AI指標を含むコミット履歴を表示します。

```bash
$ git ai-trail log --since="1 week ago"
a1b2c3d 認証を追加
  Author: Alice
  AI: 120 lines (75.0%) - 90 ai, 0 ai-modified, 30 human
  Mark: model=gpt-4 agent=cursor at=2024-01-15 10:30:00

b2c3d4e バグ修正
  Author: Bob
  AI: 帰属データなし
```

### `git ai-trail show [<commit>]`

コミットの生の帰属記録を表示します。

```bash
$ git ai-trail show
Commit: a1b2c3d4e5f6
Schema Version: 1
Tool Version: 0.1.0

Marks:
  [1]
    Timestamp: 2024-01-15T10:30:00Z
    Model: gpt-4
    Agent: cursor
    Prompt Hash: 7d8e9f...

Files:
  src/main.go: 60 lines (75.0% AI)
    45 ai, 3 ai-modified, 12 human

Total: 60 lines (45 ai, 3 ai-modified, 12 human) - 80.0% AI
```

### `git ai-trail init`

AI追跡のためにリポジトリを初期化します。これにより以下が設定されます:
- notesの書き換え設定（amend/rebaseで保持）
- `refs/notes/ai-trail`のfetch refspec

```bash
git ai-trail init
```

## 帰属スキーマ

帰属データは`refs/notes/ai-trail`配下のgit notesにJSONとして保存されます。

**スキーマ（v1）:**
```json
{
  "schema_version": 1,
  "tool_version": "0.1.0",
  "files": {
    "src/main.go": {
      "path": "src/main.go",
      "ranges": [
        {"start": 1, "end": 10, "kind": "ai"},
        {"start": 11, "end": 15, "kind": "ai-modified"},
        {"start": 16, "end": 20, "kind": "human"}
      ]
    }
  },
  "marks": [
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "model": "gpt-4",
      "agent": "cursor",
      "prompt_hash": "7d8e9f1a2b3c...",
      "prompt_short": "OAuthログインフローを実装..."
    }
  ]
}
```

**行の種類:**
- `ai`: このコミットで追加されたAI生成行、変更なし
- `ai-modified`: このコミットで追加されたAI生成行、その後人間が編集
- `human`: このコミットで追加された人間作成行、または既存行（AIには帰属されない）

**重要**: コミット（対親コミット）で追加または変更された行のみが分類されます。既存の変更されていない行はカウントされません。

## チームとの共有

AI帰属notesをプッシュしてチームと共有します:

```bash
# notesをプッシュ
git push origin refs/notes/ai-trail

# notesをプル
git fetch origin refs/notes/ai-trail:refs/notes/ai-trail
```

`init`コマンドはfetchを自動的に設定します。チームメンバーは以下を実行する必要があります:

```bash
git ai-trail init
git fetch
```

## エディタ/エージェント統合

### Cursor

Cursorは `.cursor/hooks.json`（プロジェクト）または `~/.cursor/hooks.json`（ユーザー）でフックをサポートします。`afterFileEdit` フックはエージェントがファイルを編集した後に実行されます。

**`.cursor/hooks.json`:**
```json
{
  "version": 1,
  "hooks": {
    "afterFileEdit": [
      {
        "command": ".cursor/hooks/mark-ai-changes.sh"
      }
    ]
  }
}
```

**`.cursor/hooks/mark-ai-changes.sh`:**
```bash
#!/bin/bash
git ai-trail mark --stdin-json --quiet --agent cursor
```

実行可能にする: `chmod +x .cursor/hooks/mark-ai-changes.sh`

フックはstdinでJSONを受け取り、`file_path`が含まれます。`--stdin-json`はこれを読み取って編集されたファイル（新規ファイルを含む）をキャプチャします。

参考: [Cursor Hooks ドキュメント](https://cursor.com/docs/hooks)

### Claude Code (CLI)

Claude Codeは `.claude/settings.json`（プロジェクト）または `~/.claude/settings.json`（ユーザー）でフックをサポートします。`PostToolUse` フックはツール実行後に実行されます。

**`.claude/settings.json`:**
```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|MultiEdit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "git ai-trail mark --stdin-json --quiet --agent claude-code"
          }
        ]
      }
    ]
  }
}
```

`matcher` はファイル編集ツールのみをフィルタします。フックはstdinで`tool_input.file_path`を含むイベントJSONを受け取り、`--stdin-json`はこれを読み取って編集されたファイル（`Write`で新規作成されたファイルを含む）をキャプチャします。

参考: [Claude Code Hooks ドキュメント](https://code.claude.com/docs/en/hooks)

### GitHub Copilot (手動ワークフロー)

GitHub Copilot（IDE拡張機能）は自動的な編集後フックをサポートしていません。手動ワークフローを使用します:

1. post-commitフックをインストール:
   ```bash
   git ai-trail install-hooks
   ```

2. Copilot使用後、コミット前に変更をマーク:
   ```bash
   git ai-trail mark --agent copilot
   git commit -m "..."
   ```

**注**: GitHub Copilot CLIは `.github/hooks/copilot-cli-policy.json` でフックをサポートしていますが、これはCLIツールのみに適用され、IDE拡張機能には適用されません。CLIを使用している場合は [Copilot CLI Hooks リファレンス](https://docs.github.com/en/copilot/reference/hooks-reference) を参照してください。

## 設定

### ファジーマッチング閾値

ツールはファジーマッチングを使用して、AI生成行が軽く編集されたことを検出します。類似度の閾値（0.0〜1.0、デフォルト: 0.5）を設定できます:

```bash
# 閾値を0.6に設定（ai-modified検出に60%の類似度が必要）
git config ai-trail.fuzzyThreshold 0.6

# 現在の設定を表示
git config ai-trail.fuzzyThreshold
```

- **1.0**: 完全一致のみ`ai`とカウント。編集があれば`human`になる
- **0.5**（デフォルト）: AIスナップショットとの類似度≥50%の行は`ai-modified`としてマーク
- **0.0**: 完全に異なる行でもマッチする可能性（非推奨）

**例**: AIが`function calculateTotal() {`と書き、あなたが`function calculateTotal(tax) {`に編集した場合、類似度は約85%なので`ai-modified`としてマークされます。

## ストレージの詳細

- **保留中のスナップショット**: `.git/ai-trail/*.json`に保存（コミットされない）
- **コミットされた帰属**: git notes `refs/notes/ai-trail`に保存
- **プロンプトストレージ**: SHA256ハッシュ + 最初の200文字として保存
- **worktreeサポート**: 適切なgitディレクトリ解決のために`git rev-parse --git-dir`を使用

## 制限とロードマップ

### 現在の制限

- **行マッチング**: Levenshtein類似度を使用（閾値設定可能）; 複雑なリファクタリングや移動されたコードブロックは完全には一致しない場合があります
- **単一ファイルフォーカス**: AI編集が特定のファイルに分離されている場合に最適に機能します
- **事後マッチング**: 帰属はコミット時に計算され、リアルタイムではありません
- **手動マーキング**: 明示的な`git ai-trail mark`呼び出しが必要（ただしフックで自動化可能）
- **LCSベースのアライメント**: ファイル内に多くの重複行がある場合、誤一致する可能性があります

### ロードマップ

- [ ] **検証コマンド**: 作成者がAI変更を説明するまでCIでマージをブロック
  ```bash
  # 将来の機能
  git ai-trail verify <commit> --explanation "OAuthフローをレビューして..."
  ```
- [ ] **ファジー行マッチング**: フォーマットされた/移動されたコードのより良い処理
- [ ] **IDEプラグイン**: ネイティブCursor、VS Code、IntelliJ統合
- [ ] **マージ競合解決**: スマートな帰属マージ
- [ ] **匿名モード**: プロンプト/モデルを保存せずにAI使用を追跡
- [ ] **帰属diff**: `git diff`統合でAIが触れた行を表示
- [ ] **チームダッシュボード**: プロジェクト全体のAI使用を視覚化

## 貢献

貢献を歓迎します！issueまたはPRを開いてください。

## ライセンス

MITライセンス - [LICENSE](LICENSE)を参照

## FAQ

**Q: どのAIツールでも動作しますか？**
A: はい！AI編集後に`git ai-trail mark`を呼び出す限り、どのエディタやエージェントでも動作します。

**Q: AI変更をマークし忘れたらどうなりますか？**
A: それらの行は「human」として帰属されます。安全のため、post-commitフックをインストールし、エディタフック（Cursor、Claude Code）を使用して自動的にマークします。

**Q: これはコミットを遅くしますか？**
A: `mark`コマンドは高速です（通常<100ms）。コミット時の帰属計算はファイルサイズに依存します。小〜中規模のファイルでは無視できる程度ですが、非常に大きなファイル（1000行以上）ではファジーマッチングのため数秒かかることがあります。

**Q: 既存のリポジトリで使用できますか？**
A: はい！`git ai-trail init`を実行して、今から開始してください。履歴コミットには帰属データがありません。

**Q: どれくらいのストレージを使用しますか？**
A: 非常に少ない - 帰属記録はコンパクトなJSONです。典型的なコミットの帰属は<5KBです。

**Q: プライバシーについてはどうですか？**
A: プロンプトはハッシュ化され、最初の200文字のみが保存されます。完全なプライバシーのためには、マーク時に`--prompt`を省略してください。
