# san-conv

**Convert SAN fabric zoning configurations between Cisco MDS (NX-OS) and Brocade FOS.**

`san-conv` is a single, dependency-free Go binary for ops teams migrating zoning between
Cisco MDS and Brocade switches. Give it a full MDS running-config and it produces
ready-to-apply Brocade FOS CLI commands — with clear warnings for anything it cannot
convert cleanly. Conversion is bidirectional.

## Install

Download a prebuilt binary from the
[latest release](https://github.com/fjacquet/san-conv/releases/latest), or build from
source (requires Go 1.21+):

```bash
go install github.com/fjacquet/san-conv@latest
```

## Quick start

```bash
# Cisco MDS running-config  ->  Brocade FOS commands
san-conv mds2brocade myconfig.txt

# Brocade FOS config  ->  Cisco MDS commands
san-conv brocade2mds myconfig.txt
```

See the **[User Guide](USER_GUIDE.md)** for flags, worked examples, and conversion details.

## Documentation

- **[User Guide](USER_GUIDE.md)** — installation, commands, flags, worked examples
- **[Product Requirements](PRD.md)** — what the tool does and why
- **[Design Decisions](adr/0001-go-single-binary.md)** — architecture rationale (ADRs)
- **[Changelog](changelog.md)** — release history
