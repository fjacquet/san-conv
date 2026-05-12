# san-conv User Guide

## Overview

`san-conv` converts SAN fabric zoning configurations between Cisco MDS (NX-OS) and Brocade FOS formats. The primary workflow is MDS → Brocade for fabric migrations, but the reverse direction (Brocade → MDS) is fully supported.

---

## Installation

### Option 1 — Go install (requires Go 1.21+)

```bash
go install github.com/fjacquet/san-conv@latest
```

### Option 2 — Pre-built binary

Download the appropriate binary for your platform from the [releases page](https://github.com/fjacquet/san-conv/releases):

| Platform | File |
|----------|------|
| Linux amd64 | `san-conv_linux_amd64.tar.gz` |
| Linux arm64 | `san-conv_linux_arm64.tar.gz` |
| macOS amd64 | `san-conv_darwin_amd64.tar.gz` |
| macOS arm64 (M-series) | `san-conv_darwin_arm64.tar.gz` |
| Windows amd64 | `san-conv_windows_amd64.zip` |

---

## Preparing Your Input

### MDS (NX-OS) input

Export the full running config from your MDS switch:

```
MDS-SW1# show running-config > mds-config.txt
```

Copy the file to your workstation. The tool reads it offline — no switch connectivity required.

### Brocade FOS input

Capture the zone database using `cfgshow`:

```
FOS-SW1:admin> cfgshow > brocade-config.txt
```

Or use an existing FOS CLI script containing `alicreate`, `zonecreate`, `cfgcreate` commands. The tool auto-detects the format.

---

## Basic Usage

### MDS → Brocade (default)

```bash
san-conv mds-config.txt
```

Output goes to stdout. Redirect or use `--output`:

```bash
san-conv mds-config.txt --output fos-commands.txt
```

### Brocade → MDS

```bash
san-conv brocade-config.txt --direction brocade2mds
```

### Generate a deployable shell script

The `--script` flag produces an executable script with safe FOS preamble and postamble:

```bash
san-conv mds-config.txt --output fos-commands.txt --script deploy.sh
chmod +x deploy.sh
```

The script contains:
1. `defzone --noaccess` — sets default deny-all policy
2. All `alicreate`, `zonecreate`, `cfgcreate` commands
3. `# cfgenable "CfgName"` — commented out for safety; uncomment to activate
4. `cfgsave` — saves the configuration

---

## Flags Reference

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--direction` | `-d` | `mds2brocade` | Conversion direction: `mds2brocade` or `brocade2mds` |
| `--output` | | stdout | Write primary output to file |
| `--script` | | (none) | Also write executable shell script (mds2brocade only) |
| `--fos-version` | | `8.1+` | FOS naming rules: `8.1+` (default) or `pre-8.1` (legacy switches) |

### `--fos-version` detail

FOS naming rules changed in version 8.1:

| Mode | Allowed characters | Hyphens |
|------|--------------------|---------|
| `8.1+` **(default)** | `A-Za-z0-9_$^-` | preserved |
| `pre-8.1` | `A-Za-z0-9_` | replaced with `_` |

The default is `8.1+` because FOS 8.1 shipped in 2017 and FOS 10.x is current in 2026. Use `--fos-version pre-8.1` only when targeting a switch confirmed to run FOS older than 8.1. On older switches, hyphens are silently stripped by the CLI, breaking zoning without errors.

---

## Understanding the Output

### Primary output (FOS commands)

```
alicreate "SRV_ESX01_HBA0", "10:00:00:90:fa:12:34:56"
alicreate "SRV_ESX01_HBA1", "10:00:00:90:fa:12:34:57"
...
zonecreate "ZONE_ESX01_STOR01", "SRV_ESX01_HBA0;STOR01_PORT0"
...
cfgcreate "FABRIC_A", "ZONE_ESX01_STOR01;ZONE_ESX02_STOR01"
cfgenable "FABRIC_A"
```

Commands appear in application order: aliases first, then zones, then configs.

### Stderr summary

```
WARN: alias "SRV-ESX01-HBA0": invalid characters replaced -> "SRV_ESX01_HBA0"
Summary: 48 aliases, 32 zones, 2 configs | Warnings: 3
```

All warnings are non-fatal — the tool always produces best-effort output even when warnings are present.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (warnings are allowed) |
| 1 | Fatal error (file not found, parse failure, IO error) |

### Smart zoning → peer zones

If a Cisco MDS zone uses smart zoning (members tagged `init`, `target`, or
`both`), `mds2brocade` converts it to a Brocade **peer zone**:

```
zonecreate --peerzone "ZoneName" -principal "<targets>" -members "<initiators>"
```

- `target` and `both` members, plus any member with no role tag, become
  `-principal` members; a warning is emitted for each `both`/untagged member
  (placing them as principals is connectivity-safe but slightly over-permissive).
- `init` members become non-principal `-members`.
- If a smart-zoned zone has no principal members (all `init`), it is emitted as a
  plain `zonecreate` instead, with a warning.

Peer-zone output requires **Fabric OS ≥ 7.4** on the target switch (peer zoning
was introduced in FOS 7.4). It is emitted regardless of `--fos-version`.

---

## Name Sanitization

FOS enforces stricter naming rules than NX-OS. The tool sanitizes names automatically:

### Character replacement

Characters outside the allowed set (see `--fos-version`) are replaced with `_`. A warning is emitted for each name changed:

```
WARN: alias "SRV-ESX01": invalid characters replaced -> "SRV_ESX01"
```

### Truncation

Names longer than 63 characters are truncated. The warning shows both the original and new name:

```
WARN: zone "VERY_LONG_ZONE_NAME_...": truncated to 63 characters
```

### Collision detection

If two names become identical after sanitization, the tool appends `_2`, `_3`, etc. and lists all affected originals:

```
WARN: collision: names ["SRV-ESX01" "SRV_ESX01"] all sanitize to "SRV_ESX01" -- disambiguated
```

All cross-references (zone members, zoneset membership lists) are updated to the new names automatically.

---

## Handling Unsupported Constructs

Some MDS constructs have no FOS equivalent. The tool skips them and emits a warning rather than stopping:

| Construct | Behavior |
|-----------|----------|
| `member interface` | Skipped with warning |
| `member fcid` | Skipped with warning |
| `member ip-address` | Skipped with warning |
| `member symbolic-nodename` | Skipped with warning |
| Smart zoning keywords (`init`/`target`/`both`) | Converted to a Brocade peer zone — see "Smart zoning → peer zones" below |
| IVR zones | Entire zone skipped with warning |
| TI zones | Entire zone skipped with warning |

If a zone has all members skipped, the zone itself is omitted from output (with a warning). Always review warnings before applying output to a switch.

---

## Multi-VSAN Configs

MDS uses per-VSAN zoning. Brocade uses a single fabric zone database. When converting a multi-VSAN MDS config:

- All VSANs are merged into a single Brocade zone database
- VSAN information is not preserved in output (Brocade uses separate physical or virtual fabrics)
- Zone and alias names from different VSANs may collide — the collision detection handles this with `_2`/`_3` disambiguation

For large environments with many VSANs, use `--vsan <N>` to convert one VSAN at a time.

---

## Named Subcommands

The following subcommand forms are equivalent to using `--direction`:

```bash
# These are identical:
san-conv myconfig.txt --direction mds2brocade
san-conv mds2brocade myconfig.txt

# These are identical:
san-conv myconfig.txt --direction brocade2mds
san-conv brocade2mds myconfig.txt
```

---

## Typical Migration Workflow

1. **Export** MDS running-config during a read-only review window
2. **Run** san-conv and redirect output to a file
3. **Review** warnings — investigate any unexpected skipped members
4. **Diff** aliases/zones against source to verify completeness
5. **Test** on a non-production fabric or VSAN first
6. **Apply** during the maintenance window: paste commands or run the script
7. **Verify** with `cfgshow` on the Brocade switch post-apply

---
*Last updated: 2026-03-29 — san-conv v1.0.0*
