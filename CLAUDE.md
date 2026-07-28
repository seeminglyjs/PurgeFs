# PurgeFs

Fast terminal disk cleaner for macOS/Linux. CLI (no GUI). Scan a path, purge junk files/directories. Inspired by `tw93/mole`, trimmed to a focused command tool.

## Stack

- Go (module `github.com/seeminglyjs/PurgeFs`).
- CLI framework: `cobra` (declarative command tree, easy to read/review).
- Build: `go build -o purgefs .` → single static binary. Run directly: `go run . scan PATH`.

## Layout

- `main.go` — entry, calls `cmd.Execute()`.
- `cmd/root.go` — root command, version, `Execute()`.
- `cmd/scan.go` — `scan` subcommand (stubbed).
- `cmd/purge.go` — `purge` subcommand + `--yes` flag (stubbed).

## Commands

- `purgefs scan [PATH]` — report junk, no deletion.
- `purgefs purge [PATH] [--yes]` — delete junk, confirm unless `--yes`.

## Conventions

- Deletion is dangerous. Default to dry-run/report. Never delete without explicit confirm or `--yes`.
- Keep it dependency-light. Justify each new module.
- Purge rules target build/cache junk: `target/`, `node_modules/`, `__pycache__/`, `.DS_Store`, logs, temp — never user data by default.
- New subcommand = new file under `cmd/`, register via `init()` + `rootCmd.AddCommand`.

## Git

- Commit `go.mod` and `go.sum`.
- Do NOT add Claude as co-author on commits.
