# Contributing to ZFSdash

We welcome contributions of all kinds — code, documentation, bug reports, and ideas.

## Shaping the Roadmap

**[Join the roadmap discussion →](https://github.com/zfsdash/zfsdash/discussions/12)**

Tell us your use case, what's missing, and what matters most. Decisions are made in the open.

## Filing Issues

- **Bug reports:** Include your OS, ZFS version, pool configuration, and steps to reproduce.
- **Feature requests:** Explain the problem you're trying to solve, not just the solution.
- **Good first issues:** Browse [`good first issue`](https://github.com/zfsdash/zfsdash/issues?q=label%3A%22good+first+issue%22) labels — these are well-scoped and documented.

## Submitting Code

1. Fork the repo
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Write tests for your changes
4. Ensure `go test ./...` passes
5. Ensure `go build ./cmd/zfsdash` produces a working binary
6. Open a pull request with a clear description of what and why

## Code Style

- Standard Go formatting: `gofmt -w .`
- No external dependencies without discussion — the single-binary constraint is intentional
- All public types and functions must have godoc comments
- Error handling: always wrap with `fmt.Errorf("context: %w", err)`

## Architecture

```
cmd/zfsdash/          → main entrypoint, flag parsing
internal/zfs/         → zpool/zfs command execution and parsing
internal/web/         → HTTP handlers, embedded static assets
internal/db/          → SQLite schema and queries
internal/auth/        → session management, bcrypt
internal/wizard/      → first-run setup wizard
internal/events/      → SSE event buffers for real-time streaming
internal/collector/   → ARC stats, iostat, capacity trend collectors
```

## Testing

ZFS commands are hard to unit test without real pools. We use:
- Unit tests for parsers (zpool status output parsing, scrub progress parsing)
- Integration tests with mock command output
- Manual testing on real ZFS pools before any release

To run tests:
```bash
go test ./...
```

## Release Process

Releases are tagged manually: `git tag v0.x.y && git push --tags`.

Binaries are built and uploaded to GitHub Releases for Linux amd64/arm64 and FreeBSD amd64.

## Questions?

Open a [Discussion](https://github.com/zfsdash/zfsdash/discussions) — we respond quickly.
