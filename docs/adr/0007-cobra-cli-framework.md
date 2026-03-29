# ADR-0007: Cobra as CLI framework

**Date:** 2026-03-28
**Status:** Accepted

## Context

The tool needs a CLI framework to handle subcommands (`mds2brocade`, `brocade2mds`), POSIX flags (`--direction`, `--output`, `--script`, `--fos-version`), automatic `--help` generation, and `SilenceUsage` (suppress usage on runtime errors). The stdlib `flag` package handles simple flags but not subcommands.

## Decision

Use `github.com/spf13/cobra` v1.10.2 as the CLI framework.

## Rationale

- **Subcommand support**: Cobra's command tree (`rootCmd.AddCommand(mds2brocadeCmd)`) handles subcommand routing natively.
- **Industry standard**: Used by Kubernetes, Hugo, GitHub CLI, Docker CLI. Extensive documentation and community familiarity.
- **Flat dispatch via root `RunE`**: Cobra routes `san-conv mds2brocade myfile.txt` to the named subcommand, but `san-conv myfile.txt` (no subcommand) routes to `rootCmd.RunE` — enabling the flat invocation pattern without duplicating code.
- **`SilenceUsage`**: `rootCmd.SilenceUsage = true` prevents cobra from printing usage text on runtime errors, which would pollute the stderr warning output.

## Alternatives considered

| Alternative | Rejected because |
|-------------|-----------------|
| `flag` (stdlib) | No subcommand support; would require manual routing |
| `github.com/alecthomas/kong` | Cleaner struct-tag API, but smaller ecosystem and less documentation density for a 2-3 subcommand tool |
| `github.com/urfave/cli/v3` | Viable, but cobra is the industry default; team gains from its documentation |

## Consequences

- `github.com/spf13/pflag` (cobra's flag backend) is a transitive dependency — not used directly
- Root-level flags (`--direction`, `--output`, `--script`, `--fos-version`) are declared on `rootCmd`; subcommand flags are declared on their respective commands
- Cobra routes subcommands before `rootCmd.RunE` — `san-conv mds2brocade myfile.txt` routes to `mds2brocadeCmd`, never to `rootCmd.RunE`
- `cobra.ExactArgs(1)` on `rootCmd` enforces the positional argument requirement
