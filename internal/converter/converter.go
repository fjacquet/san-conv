package converter

import (
	"bytes"
	"fmt"
	"io"
	"os"

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
