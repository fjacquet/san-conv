# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-03-29

### Changed
- `--fos-version` default changed from `pre-8.1` to `8.1+` — FOS 8.1 shipped in 2017; FOS 10.x is current; the conservative default was generating spurious hyphen-replacement warnings on every modern switch

### Added
- `docs/PRD.md` — product requirements document with v1/v2/out-of-scope tables and requirement traceability
- `docs/USER_GUIDE.md` — full user guide: installation, input preparation, usage, flags, sanitization behavior, multi-VSAN, migration workflow
- `docs/adr/` — 7 Architecture Decision Records: Go language choice, warn-and-continue policy, bidirectional design, IR compiler pipeline, two-pass MDS parser, FOS naming default, Cobra CLI framework
- `SECURITY.md` — vulnerability reporting policy and scope statement
- `.github/dependabot.yml` — weekly automated dependency updates for Go modules and GitHub Actions
- README badges: CI, Release, Go Report Card, pkg.go.dev, latest release, MIT license

### Fixed
- GitHub Actions lint job: upgraded `golangci-lint-action@v6` → `@v7` (v6 does not support golangci-lint v2.x)

## [1.0.0] - 2026-03-29

### Added
- Parse Cisco MDS NX-OS running-config: `device-alias database`, `zone name`, `zoneset name`, `zoneset activate`
- Parse Brocade FOS config: `alicreate`, `zonecreate`, `cfgcreate`, `cfgenable` (cfgshow and CLI script formats)
- FOS name sanitizer: invalid character replacement, 63-char truncation, collision disambiguation with `_2`/`_3` suffixes
- FOS version modes: `pre-8.1` (conservative charset) and `8.1+` (allows `$`, `^`, `-`)
- Brocade FOS emitter: `alicreate`, `zonecreate`, `cfgcreate`, `cfgenable` output; script mode with `defzone --noaccess` and `cfgsave`
- MDS NX-OS emitter: `device-alias database`, `zone name`, `zoneset name`, `zoneset activate` output
- Converter pipeline: parse → sanitize → emit → warnings summary to stderr
- CLI: flat invocation `san-conv myconfig.txt` (mds2brocade default) and `--direction brocade2mds`
- Flags: `--output`, `--script`, `--fos-version`, `--direction`
- Named subcommands `mds2brocade` and `brocade2mds` as aliases
- `--script` produces executable shell script (chmod 0755) with FOS deployment sequence
- stderr summary: `Summary: N aliases, N zones, N configs | Warnings: N`
- Exit 0 on warnings-only; exit 1 on IO/parse errors
- Cross-platform binaries via goreleaser: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- MIT license

### Security
- `google.golang.org/grpc` v1.79.3 (CVE: authorization bypass, critical)
- `github.com/buger/jsonparser` v1.1.2 (CVE: denial of service, high)
