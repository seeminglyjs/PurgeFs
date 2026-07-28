# PurgeFs — Design Spec

- **Date:** 2026-07-28
- **Status:** Approved (design), pending implementation plan
- **Author:** seeminglyjs

## Summary

PurgeFs is a fast terminal disk cleaner for macOS (Linux later). It is a
**hybrid** of a disk-usage analyzer and a rule-based junk cleaner: it walks a
path, aggregates sizes, matches junk patterns, and lets the user select and
purge via a TUI. Deletion defaults to the **macOS Trash** (recoverable);
permanent delete is opt-in via `--hard`.

Inspired by `tw93/mole`, but not bound to it. PurgeFs adds its own strengths:
safety-first trash + `undo`, an audit log, a developer-cache preset, reclaim
previews, and staleness awareness.

## Goals

- Command-line tool, no GUI in v1 (no paywall, no bloat).
- Hybrid: size-analysis view **and** junk-category view over the same data.
- Safe by default: trash, not permanent delete. Recoverable via `undo`.
- TUI-centric selection (arrow keys / space to check), like mole.
- Good maintainability and reviewability (Go + clean package boundaries).
- Fast enough — bottleneck is disk I/O, not CPU.

## Non-Goals (v1)

- Native GUI (deferred; architecture leaves the door open — see below).
- Linux support (interface abstracted, implementation later).
- User-defined config-file rules (built-in rules only in v1).
- Staleness *scoring*/auto-recommendation (v1 only surfaces age; scoring later).

## Stack

- Go (module `github.com/seeminglyjs/PurgeFs`).
- CLI: `cobra`. TUI: `bubbletea` (de-facto Go TUI, well-maintained, reviewable).
- Single static binary. Build: `go build -o purgefs .`

## Architecture

Two layers with a strict one-way dependency: frontends depend on the engine;
the engine never imports a frontend. This boundary is what lets a future GUI
attach as just another thin frontend.

```
┌─────────── frontends (thin) ───────────┐
│  cmd/ (CLI)      tui/ (TUI)   [GUI later]│
└──────────────────┬─────────────────────┘
                   │ depends on engine only
┌──────────────────▼─────────────────────┐
│  engine: walk → rules → Report model    │
│  trash · history                        │
└─────────────────────────────────────────┘
```

### Package Layout

```
purgefs/
  main.go
  cmd/                 cobra: root, scan, purge, undo
  internal/
    engine/
      walk.go          Walker: concurrent traversal + size aggregation
      scan.go          Scan: orchestrates walk → rules → Report
      rule.go          Rule interface + registry
      rules/           built-in rules (node_modules, buildcache, dsstore, ...)
      model.go         Entry / Report (tree view + category view, same data)
    trash/             Trasher interface + macOS impl (~/.Trash)
    history/           purge manifest record + undo restore
    tui/               bubbletea TUI (select + purge)
```

`internal/` blocks external imports so the API can evolve freely; packages can
be promoted to public once stable.

### Core Interfaces (the boundary contracts)

Frontends only need these four contracts. Swapping the TUI does not change the
engine.

- **Rule**
  ```
  Match(path string, d fs.DirEntry) (matched bool, category string, skipDir bool)
  ```
  `skipDir` lets a directory rule (e.g. `node_modules`) match the dir and stop
  descending into it.

- **Trasher**
  ```
  Trash(paths []string) (TrashResult, error)
  ```
  macOS impl moves entries to `~/.Trash`, renaming on name collision. `--hard`
  swaps in a permanent-delete implementation of the same interface.

- **Report** — a tree of entries with sizes, plus category grouping. Two views
  over one dataset; the hybrid analyzer/cleaner UX comes from this.

- **History** — records a manifest `{timestamp, entries, trashLocation}`; `undo`
  restores entries from the Trash. Same manifest doubles as the audit log.

## Differentiators (where each lives)

1. **Undo (`purgefs undo`)** — `internal/history` + `cmd/undo`. Enabled by
   trash-by-default. Restores the last purge.
2. **Dev-cache preset** — `internal/engine/rules` preset bundle + `purge
   --preset dev-caches`: sweeps `node_modules` / `target` / `build` / `.gradle`
   across a workspace in one command.
3. **Staleness awareness** — `mtime`-age field on entries in `model.go`. v1
   surfaces age; scoring/auto-recommendation is post-v1.
4. **Reclaim preview** — `Report` aggregates reclaimable size per category and
   in total, shown before any deletion.
5. **Audit log** — `internal/history` manifest. Restore evidence + traceability
   at `~/.purgefs/history`.

Undo and audit log share the history package: one manifest is both the restore
source and the log.

## Data Flow

1. `scan PATH` → Walker traverses `PATH` concurrently, aggregating sizes.
2. Each entry is offered to the Rule registry; matches are tagged with a
   category; `skipDir` prunes traversal.
3. Results build a Report (tree + category grouping + per-category reclaim
   totals).
4. `scan` prints the Report (dry-run by nature). TUI renders it for selection.
5. `purge` sends selected paths to the Trasher; the History package writes a
   manifest.
6. `undo` reads the latest manifest and restores from the Trash.

## Roadmap (incremental; each phase ships something that runs)

- **P1 — Engine skeleton:** Walker (concurrent traversal) + size aggregation +
  Report model. `scan` prints tree/summary (no TUI yet). Tests.
- **P2 — Rule system:** Rule interface + registry + built-in rules
  (`node_modules`, `target`, `build`, `.gradle`, `__pycache__`, `dist`,
  `.DS_Store`). Category grouping + reclaim preview.
- **P3 — Trash + purge:** macOS Trasher (`~/.Trash` move, collision rename) +
  `purge` with confirmation. `--hard` permanent delete.
- **P4 — TUI:** bubbletea selection UI — checkboxes, size bars, category↔tree
  toggle, confirm.
- **P5 — Differentiators:** history manifest + `undo`; `--preset dev-caches`.

**v1 = P1–P5.** macOS-only, TUI-centric, undo + preset included.

- **Post-v1:** Linux trash (XDG trash spec), config-file custom rules, Wails
  GUI, staleness scoring.

## Safety & Error Handling

Deletion is the dangerous part; safety is the product's identity.

- Never delete outside the scanned root.
- Refuse/force-confirm on dangerous roots: `/`, entire `~`, home top-level —
  anything not clearly scoped requires explicit confirmation.
- Do **not** follow symlinks — operate on the link itself, never delete targets.
- Permission errors: skip the entry and report it; never abort the whole run.
- Trash-by-default is recoverable. `--hard` (permanent) requires a separate
  confirmation.
- `scan` is inherently dry-run. `purge` always confirms in the TUI.

## Testing (TDD)

- **Unit:** rules (table tests), Walker (temp-dir fixtures), Trasher (temp
  `HOME`), History (record ↔ restore round-trip).
- **Integration:** scan a fixture tree, assert the Report; purge → undo
  round-trip restores.
- Tests written before implementation for each phase.

## Open Questions

- None blocking. Config-file rule format and Linux trash details are deferred to
  post-v1 and will get their own specs.
