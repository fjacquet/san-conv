package converter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	brocadeemitter "github.com/fjacquet/san-conv/internal/emitter/brocade"
	mdsemitter "github.com/fjacquet/san-conv/internal/emitter/mds"
	"github.com/fjacquet/san-conv/internal/ir"
	brocadeparser "github.com/fjacquet/san-conv/internal/parser/brocade"
	mdsparser "github.com/fjacquet/san-conv/internal/parser/mds"
	"github.com/fjacquet/san-conv/internal/validator"
)

// Options holds all configuration for a single converter.Run invocation.
type Options struct {
	InputFile  string
	Direction  string
	OutputFile string
	ScriptFile string
	FOSVersion string
	VSAN       int // when non-zero, convert only this VSAN's zones/zonesets (mds2brocade only)
}

// Run executes the full conversion pipeline: parse → validate → emit.
// Primary output goes to stdout unless OutputFile is set.
// When ScriptFile is set (mds2brocade only), an executable shell script is also written.
// Warnings and a summary line are written to stderr.
// Run never calls os.Exit; errors are returned to the caller.
func Run(opts Options, stdout io.Writer, stderr io.Writer) error {
	// Step 1: Open input file.
	f, err := os.Open(opts.InputFile)
	if err != nil {
		return fmt.Errorf("open %q: %w", opts.InputFile, err)
	}
	defer f.Close()

	// Step 2: Parse based on direction.
	var cfg *ir.ZoningConfig
	switch opts.Direction {
	case "mds2brocade":
		parsed, parseErr := mdsparser.Parse(f)
		if parseErr != nil {
			return fmt.Errorf("parse: %w", parseErr)
		}
		cfg = parsed
	case "brocade2mds":
		parsed, parseErr := brocadeparser.Parse(f)
		if parseErr != nil {
			return fmt.Errorf("parse: %w", parseErr)
		}
		cfg = parsed
	default:
		return fmt.Errorf("unknown direction %q (use mds2brocade or brocade2mds)", opts.Direction)
	}

	// Step 2b: Optionally scope to a single VSAN (mds2brocade only).
	if opts.Direction == "mds2brocade" && opts.VSAN != 0 {
		filterVSAN(cfg, opts.VSAN)
	}

	// Step 3: Sanitize ONLY for mds2brocade direction.
	if opts.Direction == "mds2brocade" {
		cfg = validator.Sanitize(cfg, opts.FOSVersion)
	}

	// Step 4: Resolve primary output writer.
	primaryW := stdout
	if opts.OutputFile != "" {
		of, ferr := os.Create(opts.OutputFile)
		if ferr != nil {
			return fmt.Errorf("create output %q: %w", opts.OutputFile, ferr)
		}
		defer of.Close()
		primaryW = of
	}

	// Step 5: Emit primary output and handle optional script file.
	switch opts.Direction {
	case "mds2brocade":
		if err := brocadeemitter.Emit(cfg, primaryW, false); err != nil {
			return fmt.Errorf("emit: %w", err)
		}
		// Script emit: snapshot warning count BEFORE second emit to avoid double warnings.
		if opts.ScriptFile != "" {
			warnCountBefore := len(cfg.Warnings)
			var scriptBuf bytes.Buffer
			if err := brocadeemitter.Emit(cfg, &scriptBuf, true); err != nil {
				return fmt.Errorf("emit script: %w", err)
			}
			// Trim any new warnings added by script emit (they duplicate plain emit warnings).
			cfg.Warnings = cfg.Warnings[:warnCountBefore]
			sf, serr := os.OpenFile(opts.ScriptFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if serr != nil {
				return fmt.Errorf("create script %q: %w", opts.ScriptFile, serr)
			}
			defer sf.Close()
			if _, err := scriptBuf.WriteTo(sf); err != nil {
				return fmt.Errorf("write script: %w", err)
			}
		}
	case "brocade2mds":
		if err := mdsemitter.Emit(cfg, primaryW); err != nil {
			return fmt.Errorf("emit: %w", err)
		}
	}

	// Step 6: Print warnings and summary to stderr — AFTER all emit calls.
	for _, w := range cfg.Warnings {
		fmt.Fprintf(stderr, "WARN: %s\n", w)
	}
	fmt.Fprintf(stderr, "Summary: %d aliases, %d zones, %d configs converted; %d warnings\n",
		len(cfg.Aliases), len(cfg.Zones), len(cfg.ZoneConfigs), len(cfg.Warnings))

	return nil
}

// filterVSAN removes everything that does not belong to the given VSAN: zones
// and zonesets whose VSAN differs, and fcaliases whose VSAN is neither 0 nor
// vsan. Device-aliases (VSAN 0, fabric-wide) are always kept. If filtering
// leaves no zones, a warning is appended to cfg.Warnings.
func filterVSAN(cfg *ir.ZoningConfig, vsan int) {
	for key, z := range cfg.Zones {
		if z.VSAN != vsan {
			delete(cfg.Zones, key)
		}
	}
	for key, zc := range cfg.ZoneConfigs {
		if zc.VSAN != vsan {
			delete(cfg.ZoneConfigs, key)
		}
	}
	for name, a := range cfg.Aliases {
		if a.VSAN != 0 && a.VSAN != vsan {
			delete(cfg.Aliases, name)
		}
	}

	// The parser's "multi-VSAN input: … pass --vsan N to scope" advice no longer
	// applies once we have scoped — rewrite its tail to reflect what we did.
	for i, w := range cfg.Warnings {
		if !strings.HasPrefix(w, "multi-VSAN input:") {
			continue
		}
		if before, _, found := strings.Cut(w, " — "); found {
			cfg.Warnings[i] = before + fmt.Sprintf(" — converted only VSAN %d (--vsan)", vsan)
		}
	}

	if len(cfg.Zones) == 0 {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"--vsan %d matched no zones in the input; check the VSAN number", vsan))
	}
}
