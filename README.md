# PurgeFs

Fast terminal disk cleaner for macOS and Linux. Scan a path, see what's wasting space, purge the junk — no GUI, no paywall.

Inspired by [tw93/mole](https://github.com/tw93/mole), trimmed down to a focused CLI.

## Status

Early scaffold. `scan` / `purge` commands are stubbed — not implemented yet.

## Build

Requires Go (1.24+).

```bash
# install Go if missing
brew install go

go build -o purgefs .
./purgefs --help
```

Or run without building:

```bash
go run . scan ~/Downloads
```

## Usage

```bash
purgefs scan [PATH]            # report junk under PATH (default: .), no deletion
purgefs purge [PATH] [--yes]   # delete junk under PATH, asks first unless --yes
```

## Layout

```
main.go        entry point
cmd/           cobra command tree
  root.go      root command + version + Execute()
  scan.go      scan subcommand
  purge.go     purge subcommand
```

## Roadmap

- [ ] Filesystem walk with size aggregation (`filepath.WalkDir`)
- [ ] Junk rules (build caches, node_modules, __pycache__, .DS_Store, logs, temp)
- [ ] Interactive TUI selection
- [ ] Dry-run + confirmation before delete
- [ ] Config file for custom rules / ignore paths

## License

MIT — see [LICENSE](LICENSE).
