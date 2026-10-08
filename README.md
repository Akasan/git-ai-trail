# git-ai-trail

Track AI-generated code changes in Git to maintain an audit trail of AI assistance.

[日本語版 README はこちら](README.ja.md)

## Motivation

In the AI era, it's increasingly important to:
- **Track provenance**: Know which code was written by AI vs humans
- **Ensure understanding**: Make sure developers understand AI-generated code they merge
- **Audit trail**: Maintain records for compliance, security reviews, and code quality

`git-ai-trail` helps teams keep an audit trail of AI-generated code, enabling future tooling that can block merges until the author demonstrates understanding of AI-written changes.

## Installation

```bash
go install github.com/Akasan/git-ai-trail@latest
```

Or build from source:

```bash
git clone https://github.com/Akasan/git-ai-trail
cd git-ai-trail
go build -o git-ai-trail
sudo mv git-ai-trail /usr/local/bin/
```

## Quick Start

1. **Initialize your repository**:
   ```bash
   git ai-trail init
   ```

2. **Mark AI-generated changes**:
   ```bash
   # After AI edits files, snapshot them
   git ai-trail mark --model gpt-4 --agent cursor
   ```

3. **Commit with attribution**:
   ```bash
   git ai-trail commit -m "Add feature with AI assistance"
   ```

4. **View AI attribution**:
   ```bash
   # Check uncommitted changes
   git ai-trail status
   
   # See attribution history
   git ai-trail log
   
   # Blame with AI context
   git ai-trail blame src/main.go
   ```

## Commands

### `git ai-trail mark [options] [paths...]`

Record current file changes as AI-generated. Call this immediately after AI edits files.

**Options:**
- `--model NAME`: AI model name (e.g., `gpt-4`, `claude-3`)
- `--agent NAME`: AI agent/editor name (e.g., `cursor`, `github-copilot`)
- `--prompt TEXT`: Prompt text (truncated to 200 chars)
- `--prompt-file PATH`: Read prompt from file
- `--stdin-json`: Read file path from JSON on stdin (for editor hooks that provide `tool_input.file_path`)
- `--quiet`, `-q`: Suppress output

**Examples:**
```bash
# Mark all changed files
git ai-trail mark --model gpt-4 --agent cursor

# Mark specific files
git ai-trail mark src/main.go src/util.go

# Include prompt context
git ai-trail mark --prompt "Implement OAuth login" --model gpt-4
```

### `git ai-trail status`

Show AI attribution for uncommitted changes.

```bash
$ git ai-trail status
AI attribution for uncommitted changes:

src/main.go:
  45 ai, 3 ai-modified, 12 human (80.0% AI)

Overall: 45 ai, 3 ai-modified, 12 human (80.0% AI)
```

### `git ai-trail commit [git commit args...]`

Commit changes and automatically record AI attribution in git notes.

```bash
git ai-trail commit -m "Add new feature"
```

### `git ai-trail record [<commit>]`

Add attribution record to an existing commit. Useful in post-commit hooks.

```bash
# Record attribution for HEAD
git ai-trail record

# Record for specific commit
git ai-trail record abc123
```

### `git ai-trail install-hooks`

Install post-commit hook to automatically record attribution.

```bash
git ai-trail install-hooks
```

After installation, you only need to run `git ai-trail mark` before committing; attribution will be recorded automatically.

### `git ai-trail blame <file>`

Show line-by-line AI attribution (like `git blame`).

```bash
$ git ai-trail blame src/main.go
a1b2c3d AI    1) package main
a1b2c3d AI    2) 
a1b2c3d AI*   3) func main() {
a1b2c3d HUM   4)     println("hello")
```

Labels:
- `AI`: AI-generated, unchanged
- `AI*`: AI-generated, then modified by human
- `HUM`: Human-written

### `git ai-trail log [git log args...]`

Show commit history with AI metrics.

```bash
$ git ai-trail log --since="1 week ago"
a1b2c3d Add authentication
  Author: Alice
  AI: 120 lines (75.0%) - 90 ai, 0 ai-modified, 30 human
  Mark: model=gpt-4 agent=cursor at=2024-01-15 10:30:00

b2c3d4e Fix bug
  Author: Bob
  AI: No attribution data
```

### `git ai-trail show [<commit>]`

Show raw attribution record for a commit.

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

Initialize repository for AI tracking. This configures:
- Notes rewrite settings (survive amend/rebase)
- Fetch refspec for `refs/notes/ai-trail`

```bash
git ai-trail init
```

## Attribution Schema

Attribution data is stored as JSON in git notes under `refs/notes/ai-trail`.

**Schema (v1):**
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
      "prompt_short": "Implement OAuth login flow with..."
    }
  ]
}
```

**Line kinds:**
- `ai`: AI-generated line added in this commit, unchanged
- `ai-modified`: AI-generated line added in this commit, then edited by human
- `human`: Human-written line added in this commit, or pre-existing line (not attributed to AI)

**Important**: Only lines added or changed in the commit (vs parent) are classified. Pre-existing unchanged lines are not counted.

## Sharing with Your Team

Push AI attribution notes to share with your team:

```bash
# Push notes
git push origin refs/notes/ai-trail

# Pull notes
git fetch origin refs/notes/ai-trail:refs/notes/ai-trail
```

The `init` command configures fetch automatically. Team members should run:

```bash
git ai-trail init
git fetch
```

## Editor/Agent Integration

### Cursor

Cursor supports hooks via `.cursor/hooks.json` (project) or `~/.cursor/hooks.json` (user). The `afterFileEdit` hook runs after the agent edits files.

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

Make it executable: `chmod +x .cursor/hooks/mark-ai-changes.sh`

The hook receives JSON on stdin with `file_path`, which `--stdin-json` reads to capture the edited file (including new files).

Reference: [Cursor Hooks Documentation](https://cursor.com/docs/hooks)

### Claude Code (CLI)

Claude Code supports hooks via `.claude/settings.json` (project) or `~/.claude/settings.json` (user). The `PostToolUse` hook runs after tool execution.

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

The `matcher` filters to file-editing tools only. The hook receives event JSON on stdin with `tool_input.file_path`, which `--stdin-json` reads to capture the edited file (including newly created files from `Write`).

Reference: [Claude Code Hooks Documentation](https://code.claude.com/docs/en/hooks)

### GitHub Copilot (Manual Workflow)

GitHub Copilot (IDE extension) does not support automatic post-edit hooks. Use a manual workflow:

1. Install the post-commit hook:
   ```bash
   git ai-trail install-hooks
   ```

2. After using Copilot, mark changes before committing:
   ```bash
   git ai-trail mark --agent copilot
   git commit -m "..."
   ```

**Note**: GitHub Copilot CLI has hook support via `.github/hooks/copilot-cli-policy.json`, but this applies only to the CLI tool, not the IDE extension. See [Copilot CLI Hooks Reference](https://docs.github.com/en/copilot/reference/hooks-reference) if using the CLI.

## Configuration

### Fuzzy Matching Threshold

The tool uses fuzzy matching to detect when AI-generated lines have been lightly edited. You can configure the similarity threshold (0.0 to 1.0, default: 0.5):

```bash
# Set threshold to 0.6 (60% similarity required for ai-modified detection)
git config ai-trail.fuzzyThreshold 0.6

# View current setting
git config ai-trail.fuzzyThreshold
```

- **1.0**: Only exact matches count as `ai`; any edit becomes `human`
- **0.5** (default): Lines with ≥50% similarity to AI snapshot marked as `ai-modified`
- **0.0**: Even completely different lines could be matched (not recommended)

**Example**: If AI writes `function calculateTotal() {` and you edit it to `function calculateTotal(tax) {`, the similarity is ~85%, so it's marked as `ai-modified`.

## Storage Details

- **Pending snapshots**: Stored in `.git/ai-trail/*.json` (never committed)
- **Committed attribution**: Stored in git notes `refs/notes/ai-trail`
- **Prompt storage**: Stored as SHA256 hash + first 200 characters
- **Worktree support**: Uses `git rev-parse --git-dir` for proper git dir resolution

## Limitations & Roadmap

### Current Limitations

- **Pre-AI human edits**: If you manually edit a file and then run an AI tool on the same file (without committing the manual changes first), those manual edits may be attributed to the AI. **Workaround**: Commit or stash human changes before running AI tools, or use `--stdin-json` hooks that capture only the AI's exact edits (planned: parse hook payload text to attribute only AI-written lines)
- **Line matching**: Uses Levenshtein similarity (configurable threshold); complex refactoring or moved code blocks may not match perfectly
- **Single-file focus**: Works best when AI edits are isolated to specific files
- **Post-hoc matching**: Attribution is computed at commit time, not in real-time
- **Manual marking**: Requires explicit `git ai-trail mark` calls (though hooks can automate this)
- **LCS-based alignment**: May mismatch when many duplicate lines exist in a file

### Roadmap

- [ ] **Verify command**: Block merges in CI until author explains AI changes
  ```bash
  # Future feature
  git ai-trail verify <commit> --explanation "I reviewed the OAuth flow and..."
  ```
- [ ] **Improved matching**: Better handling of moved/refactored code blocks
- [ ] **IDE plugins**: Native VS Code, IntelliJ integration
- [ ] **Merge conflict resolution**: Smart attribution merging
- [ ] **Anonymous mode**: Track AI usage without storing prompts/models
- [ ] **Attribution diffs**: `git diff` integration showing AI-touched lines
- [ ] **Team dashboards**: Visualize AI usage across projects

## Contributing

Contributions welcome! Please open an issue or PR.

## License

MIT License - see [LICENSE](LICENSE)

## FAQ

**Q: Does this work with any AI tool?**
A: Yes! As long as you call `git ai-trail mark` after AI edits, it works with any editor or agent.

**Q: What if I forget to mark AI changes?**
A: Those lines will be attributed as "human". For safety, install the post-commit hook, and use editor hooks (Cursor, Claude Code) to mark automatically.

**Q: Does this slow down commits?**
A: The `mark` command is fast (typically <100ms). Attribution computation during commit depends on file size; for small to medium files it's negligible, but very large files (1000+ lines) may take a few seconds due to fuzzy matching.

**Q: Can I use this on existing repos?**
A: Yes! Run `git ai-trail init` and start marking from now on. Historical commits won't have attribution data.

**Q: How much storage does this use?**
A: Very little - attribution records are compact JSON. A typical commit's attribution is <5KB.

**Q: What about privacy?**
A: Prompts are hashed and only the first 200 chars are stored. For full privacy, omit `--prompt` when marking.
