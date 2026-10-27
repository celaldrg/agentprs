# Changelog

## v0.1.0 (2026-10-27)

First release.

- `agentprs [owner/repo | url]`: share, outcomes, merge rate and median time to merge of AI agent PRs vs human PRs
- 14 agents recognised from the GitHub App that opened the PR or a signature in the PR body: Copilot, Codex, Claude Code, Cursor, Devin, Jules, Kiro, Amazon Q, Factory, cto.new, Codegen, CodeRabbit, OpenHands, Sweep
- Dependency bots and other bots reported separately
- Monthly agent-share sparkline and a one-line verdict
- Outputs: terminal, `--json`, `--markdown`, `--card` (SVG report card), `--badge` (SVG badge)
- `--limit`, `--since`, `--input` (offline), `--list-agents`; reads the `origin` remote when no repo is given
- Composite GitHub Action
- Single static binary, Go standard library only; prebuilt binaries for Linux, macOS and Windows
