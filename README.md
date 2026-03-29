# san-conv

Convert SAN fabric zoning configurations between Cisco MDS (NX-OS) and Brocade FOS formats.

## Install

```bash
go install github.com/fjacquet/san-conv@latest
```

Or download a pre-built binary from the [releases page](https://github.com/fjacquet/san-conv/releases).

## Usage

```bash
# MDS → Brocade (default)
san-conv myconfig.txt

# Brocade → MDS
san-conv cfgshow.txt --direction brocade2mds

# Write output to file instead of stdout
san-conv myconfig.txt --output fos-commands.txt

# Also write an executable shell script
san-conv myconfig.txt --output fos-commands.txt --script deploy.sh

# Target FOS 8.1+ naming rules (allows $, ^, - in names)
san-conv myconfig.txt --fos-version 8.1+
```

Named subcommands are also supported:

```bash
san-conv mds2brocade myconfig.txt
san-conv brocade2mds cfgshow.txt
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--direction`, `-d` | `mds2brocade` | Conversion direction: `mds2brocade` or `brocade2mds` |
| `--output` | stdout | Write primary output to file |
| `--script` | (disabled) | Also write an executable shell script (mds2brocade only) |
| `--fos-version` | `pre-8.1` | FOS naming rules: `pre-8.1` (conservative) or `8.1+` (extended) |

## Input formats

**MDS (NX-OS):** Full output of `show running-config` from a Cisco MDS switch. The tool extracts `device-alias database`, `zone name`, `zoneset name`, and `zoneset activate` blocks.

**Brocade FOS:** Output of `cfgshow` or a FOS CLI script containing `alicreate`, `zonecreate`, `cfgcreate`, and `cfgenable` commands.

## Output

Primary output (`--output` or stdout) contains ready-to-paste FOS CLI commands or NX-OS CLI commands depending on direction.

The optional script (`--script`) produces an executable shell script wrapping the same commands, with `defzone --noaccess` preamble and `cfgsave` postamble for a safe FOS deployment sequence.

A summary is always written to stderr:

```
Summary: 24 aliases, 18 zones, 2 configs | Warnings: 0
```

Non-fatal warnings (name sanitization, skipped constructs) are listed on stderr before the summary.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including warnings-only runs) |
| 1 | IO error or fatal parse failure |

## Build from source

```bash
git clone https://github.com/fjacquet/san-conv.git
cd san-conv
make build          # produces ./san-conv
make test           # run test suite
make lint           # golangci-lint
make snapshot       # goreleaser cross-platform snapshot build
```

## License

MIT — see [LICENSE](LICENSE).
