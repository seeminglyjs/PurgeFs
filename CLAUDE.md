# PurgeFs

Fast terminal disk cleaner for macOS/Linux. CLI (no GUI). Scan a path, purge junk files/directories. Inspired by `tw93/mole`, trimmed to a focused command tool.

## Stack

- Rust (edition 2021), single binary `purgefs`.
- CLI parsing: `clap` (derive).
- Build: `cargo build --release` → `target/release/purgefs`.
- Toolchain not yet installed on this machine: `brew install rust`.

## Layout

- `src/main.rs` — entry, `clap` CLI, subcommands `scan` / `purge` (both stubbed).
- `Cargo.toml` — package + deps + release profile (lto, strip).
- `README.md` — usage + roadmap.

## Commands

- `purgefs scan [PATH]` — report junk, no deletion.
- `purgefs purge [PATH] [--yes]` — delete junk, confirm unless `--yes`.

## Conventions

- Deletion is dangerous. Default to dry-run/report. Never delete without explicit confirm or `--yes`.
- Keep it dependency-light. Justify each new crate.
- Purge rules target build/cache junk: `target/`, `node_modules/`, `__pycache__/`, `.DS_Store`, logs, temp — never user data by default.

## Git

- Commit `Cargo.lock` (this is a binary, not a library).
- Do NOT add Claude as co-author on commits.
