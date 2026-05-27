# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0] - 2026-05-27

### Added

- `san-conv --version` now reports the build version and commit (e.g. `san-conv version v1.3.0 (commit abc1234)`), and a `--version` flag appears in `-h` output. The `Makefile` ldflags (`-X main.version` / `-X main.commit`) were previously no-ops because no matching package variables existed; `main.go` now declares them and `cmd.Execute` sets cobra's `rootCmd.Version`.

## [1.2.2] - 2026-05-27

### Added

- Documentation site published with MkDocs Material at <https://fjacquet.github.io/san-conv/>, built from the existing `docs/` tree (User Guide, PRD, Security, ADRs, Changelog) and deployed via GitHub Actions (`.github/workflows/docs.yml`) on every push to `main`. README gains Docs, Go-version, and Documentation badges.

### Note

- Documentation/CI only — the `san-conv` binary is byte-for-byte equivalent to v1.2.1 (no source changes since that release).

## [1.2.1] - 2026-05-27

### Changed

- Bumped the build toolchain (`goreleaser` v2.15.4 → v2.16.0), which pulls patched versions of its transitive dependencies — `go-git` v5.19.1, `go-billy` v5.9.0, `modelcontextprotocol/registry` v1.7.9, `slack-go` v0.23.1, `in-toto-golang` v0.11.0 — clearing 12 Dependabot advisories. These are **build-time-only** dependencies: the shipped `san-conv` binary contains only `cobra`/`pflag` and was never affected (verified with `go version -m` and `govulncheck`).

## [1.2.0] - 2026-05-27

### Added

- Release artifacts now ship a Software Bill of Materials (SBOM). GoReleaser generates an SPDX-JSON SBOM (via [syft](https://github.com/anchore/syft)) for every release archive and for the source tarball, published alongside the binaries and `checksums.txt`. This supports downstream vulnerability scanning and supply-chain verification. The release workflow installs syft via `anchore/sbom-action/download-syft`.

## [1.1.1] - 2026-05-13

### Added

- `brocade2mds` now accepts `--peer-consolidate` as a hidden, non-breaking alias of `--smart-consolidate`, for symmetry with `mds2brocade --peer-consolidate`. `--smart-consolidate` remains the canonical name shown in `--help` (accurate to the NX-OS *smart zoning* construct emitted); either spelling triggers the same code path. Documented in USER_GUIDE.md.

## [1.1.0] - 2026-05-13

### Added

#### Real-world MDS config robustness (Group A — ADR/spec 2026-05-11)
- `internal/preprocess` package: strips ANSI/VT100 escape sequences, normalizes `\r` to `\n`, and drops standalone `--More--` pager prompts before parsing. Both parsers now read input via `preprocess.Clean`, so terminal captures with paged output no longer silently lose zone members glued to pager prompts.
- `--vsan N` flag (`mds2brocade` only): scope conversion to a single VSAN. Zones, zonesets, and fcaliases outside that VSAN are pruned after parsing; device-aliases are fabric-wide and always kept. `0` (default) keeps the existing merge-all behavior.
- Multi-VSAN diagnostics: parser now emits a per-VSAN breakdown (zone/zoneset counts) and warns when the same zone name appears in two VSANs (Brocade has a flat zone namespace).

#### Smart-zoning ↔ peer-zoning bridge (Groups B1+B3 — ADR-0008, ADR-0010)
- `ir.ZoneMember.Role` (`""` | `"init"` | `"target"` | `"both"`). The MDS parser captures roles on member lines instead of stripping them with a warning.
- Brocade emitter renders any role-bearing zone as `zonecreate --peerzone "name" -principal "<target+both+roleless>" -members "<init>"`. A zone with no principal members falls back to a plain `zonecreate` with an informational warning.
- Brocade parser now parses `zonecreate --peerzone` (CLI) and the cfgshow `00:02:00:00:NN:NN:00:00` property marker into role-bearing `ir.Zone`s (`-principal` → `target`, non-principal → `init`; marker decode is best-effort with a plain-zone + warning fallback on bad counts).
- MDS emitter renders any role-bearing zone as `member <type> <val> <role>` plus a per-VSAN `zone smart-zoning enable vsan N` directive. Plain zones and zone-configs without any peer zones emit byte-identical to before. Round-trips `mds2brocade` → `brocade2mds` cleanly; the only loss is `both` → `target` (a Brocade peer zone has no "reaches everyone" slot).

#### Peer-zone / smart-zone consolidation (Groups B2+B4 — ADR-0009, ADR-0011)
- `internal/consolidator` package: `Consolidate(cfg, strict bool, nameSuffix string) Report` infers `(init, target)` for flat 2-member alias zones using zone-name decomposition (default = trailing-component classifier; `--consolidate-strict` reverts to exact `<host>_<target>`) plus a cross-zone frequency veto, then groups by target and rewrites the IR into per-target role-bearing zones named `<target>_<suffix>`. Direction-agnostic IR; emitter renders accordingly.
- `mds2brocade --peer-consolidate`: collapses flat single-initiator/single-target zones into per-target Brocade peer zones (`<target>_peerzone`).
- `brocade2mds --smart-consolidate`: mirror of the above — collapses flat Brocade zones into per-target MDS smart zones (`<target>_smartzone`).
- Shared flags: `--consolidate-report <file>` writes a per-merged-zone / per-skipped-zone verification report with direction-specific vocabulary; `--consolidate-strict` opts out of the trailing-component classifier.
- `internal/hygiene` package: `Check(cfg)` runs unconditionally and appends warnings for dangling alias references, empty/single-member zones, aliases defined but unused, and zones not in any zoneset. Static checks only — cannot see physical connectivity.

#### Documentation
- ADRs 0008–0011 (peer-zoning emitter, peer-zone consolidation, peer-zone round-trip, brocade-flat-to-mds-smart).
- USER_GUIDE: new sections on peer-zone output and FOS ≥ 7.4 requirement, `--vsan` scoping, peer-zone consolidation, brocade-to-smart consolidation, config hygiene warnings.

### Changed

- ADR-0002's "smart zoning has no FOS equivalent" claim is amended: peer-zone emission is the FOS equivalent.
- Consolidator API: `Consolidate(cfg, strict, nameSuffix)` is the bidirectional signature; `PeerZoneSummary` → `ConsolidatedZoneSummary` (`PeerName` → `NewName`); `Report.PeerZones` → `Report.Zones`. Internal-only — no external consumers.
- Dev toolchain bumped: goreleaser `v2.14.3` → `v2.15.4`, golangci-lint `v2.11.4` → `v2.12.2`. Runtime dependency graph unchanged.

### Fixed

- MDS parser multi-VSAN breakdown counts are derived from the deduplicated IR maps, so a config that appears twice in the input (e.g. `show zoneset active` + `show running-config`) is counted once instead of doubled.
- When `--vsan N` is given, the parser's "multi-VSAN input: … pass `--vsan N` to scope to one" warning tail is rewritten to "… converted only VSAN N (--vsan)".
- `.golangci.yml`: dropped invalid `issues.exclude-dirs` key (removed from v2 schema; this had been failing the CI Lint job).
- errcheck excludes `fmt.Fprint*`, `fmt.Sscanf`, and `(*os.File).Close` — intentional and idiomatic for an offline CLI tool.

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
