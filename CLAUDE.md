# Engineering Principles (read first — bind every agent working here)

Pre-planning and alignment beat "more tokens, no review." Bad code is not
purified by longer loops. Design and align first, then let code be written
fast and reviewed by a human. These rules are non-negotiable.

### 1. Think Before Coding
Don't assume. Don't hide confusion. Surface tradeoffs.
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First
Minimum code that solves the problem. Nothing speculative.
- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" / "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.
- Ask: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes
Touch only what you must. Clean up only your own mess.
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.
- Remove imports/vars/functions YOUR changes made unused; leave pre-existing dead code unless asked.
- Test: every changed line traces directly to the request.

### 4. Goal-Driven Execution
Define success criteria. Loop until verified.
- "Add validation" → "Write tests for invalid inputs, then make them pass."
- "Fix the bug" → "Write a test that reproduces it, then make it pass."
- "Refactor X" → "Ensure tests pass before and after."
- For multi-step tasks, state a brief plan with a verify check per step.

---

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
- **Code comments MUST be written in Korean** (한국어 주석). Applies to all `.go` source and test files — doc comments, inline comments, everything. Keep identifiers, API names, and error strings as-is.

## Git

- Commit `go.mod` and `go.sum`.
- Do NOT add Claude as co-author on commits.
