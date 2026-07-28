# PurgeFs

Fast terminal disk cleaner for macOS and Linux. Scan a path, see what's wasting space, purge the junk — no GUI, no paywall.

Inspired by [tw93/mole](https://github.com/tw93/mole), trimmed down to a focused CLI.

## Status

Early scaffold. `scan` / `purge` commands are stubbed — not implemented yet.

## Build

Requires the Rust toolchain (`rustc` + `cargo`).

```bash
# install Rust if missing
brew install rust        # or: curl https://sh.rustup.rs -sSf | sh

cargo build --release
./target/release/purgefs --help
```

## Usage

```bash
purgefs scan [PATH]            # report junk under PATH (default: .), no deletion
purgefs purge [PATH] [--yes]   # delete junk under PATH, asks first unless --yes
```

## Roadmap

- [ ] Filesystem walk with size aggregation
- [ ] Junk rules (build caches, node_modules, __pycache__, .DS_Store, logs, temp)
- [ ] Interactive TUI selection
- [ ] Dry-run + confirmation before delete
- [ ] Config file for custom rules / ignore paths

## License

MIT — see [LICENSE](LICENSE).
