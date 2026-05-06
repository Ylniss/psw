# Directory reshape: `internal/` + per-binary `cmd/` entries

Date: 2026-05-06
Status: in progress

## Motivation

Pure source code is currently spilled across the repo root (`main.go`, `cmd/`, `clipclean/`, `strg/`, `prmpt/`). We want a tidier layout, but `src/` is unidiomatic for Go. The standard layout — `cmd/<binary>/main.go` for entry points, `internal/` for private packages — keeps it idiomatic.

We also want `plans/` (this dir) tracked so design notes persist in-repo.

## Target layout

```
psw/
├── cmd/
│   ├── psw/main.go         ← was ./main.go
│   └── clipclean/main.go   ← was ./clipclean/main.go
├── internal/
│   ├── cli/                ← was ./cmd/ (cobra commands; renamed to avoid path collision)
│   ├── strg/
│   └── prmpt/
├── plans/                  ← new (this file)
├── nix/, tests/, .github/  ← unchanged
└── Makefile, go.mod, …     ← root files unchanged
```

## Steps

1. **Untrack plans** — drop `*plan.md` from `.gitignore`, create `plans/` with this file.
2. **Move source under `internal/`** — `git mv strg internal/strg`, same for `prmpt`.
3. **Rename old `cmd/` → `internal/cli/`** — also rename `package cmd` → `package cli` in every file there.
4. **Per-binary entry points under `cmd/`** — `cmd/psw/main.go` (thin wrapper around `cli.Execute()`), `cmd/clipclean/main.go` (move from top-level `clipclean/`).
5. **Build/release plumbing** — Makefile build paths + `Version` ldflag target; `nix/flake.nix` subPackages; `tests/main_test.go` build path.
6. **Docs** — refresh CLAUDE.md.

## Risks watched

- **Nix `vendorHash`**: deps unchanged, so should hold. Subpackage path changes in flake.nix only.
- **CI tarball**: workflow tars `bin/psw` + `bin/clipclean` after `make build` — those output paths are unchanged.
- **`internal/` import boundary**: single-module repo, no external importers — non-issue.
- **`Version` ldflag**: target moves from `…/cmd.Version` to `…/internal/cli.Version`. Verified via `psw --version` after build.

## Verification

- `go build ./...`
- `make build` produces `bin/psw` + `bin/clipclean`
- `make test` (integration suite)
- `bin/psw version` echoes the VERSION-file value
