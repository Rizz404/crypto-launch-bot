// catalyst performs bounded evidence collection and shadow analysis only.
// Replay uses an in-memory Sim; it never submits real exchange orders.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/rizz404/trading-bot/internal/config"
)

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("catalyst", flag.ContinueOnError)
	policyPath := flags.String("policy", "configs/catalyst.policy.json", "versioned research policy")
	dir := flags.String("archive", "var/catalyst", "isolated research archive, never a financial ledger")
	action := flags.String("action", "report", "collect, analyze, report or replay; never starts live trading")
	fixturePath := flags.String("fixture", "", "synthetic JSON fixture for replay; no network or database")
	envFile := flags.String("env-file", ".env.catalyst", "optional Plan 3 env file; process environment takes precedence")
	spiderBinary := flags.String("spider-binary", "spider", "pinned Spider CLI path for collect; no HTTP fallback")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *action == "replay" {
		f, err := os.Open(*fixturePath)
		if err != nil {
			return errors.New("replay requires a fixture file")
		}
		var fixture catalyst.Replay
		err = catalyst.Decode(f, &fixture, 16<<20)
		_ = f.Close()
		if err != nil {
			return err
		}
		result, err := fixture.Run(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(result)
	}
	f, err := os.Open(*policyPath)
	if err != nil {
		return errors.New("cannot open research policy")
	}
	var p catalyst.Policy
	err = catalyst.Decode(f, &p, 64<<10)
	_ = f.Close()
	if err != nil {
		return err
	}
	if err = p.Validate(); err != nil {
		return err
	}
	archive := catalyst.Archive{Dir: *dir}
	encode := json.NewEncoder(out)
	switch *action {
	case "collect":
		collector, err := catalyst.NewSpiderCollector(p, archive, *spiderBinary)
		if err != nil {
			return err
		}
		var failures []error
		for n, s := range p.Sources {
			if n > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
				}
			}
			e, err := collector.Collect(ctx, s)
			if err != nil {
				failures = append(failures, fmt.Errorf("source %s: %w", s.ID, err))
				continue
			}
			if err = encode.Encode(map[string]any{"source": s.ID, "evidence": e.ID, "first_seen_at": e.FirstSeenAt}); err != nil {
				return err
			}
		}
		return errors.Join(failures...)
	case "analyze":
		analyst, err := loadAnalyst(*envFile)
		if err != nil {
			return err
		}
		evidence, err := archive.LatestEvidence(p)
		if err != nil {
			return err
		}
		// One bounded request per command. No retries or model fallback here;
		// a configured OmniRoute combo may apply its own recorded routing.
		review, err := analyst.Analyze(ctx, archive, p, evidence, time.Now().UTC())
		if writeErr := encode.Encode(map[string]any{"id": review.ID, "valid": review.Valid, "action": review.Analysis.Action, "error": review.Error, "served_model": review.ServedModel, "shadow_only": true}); writeErr != nil {
			return writeErr
		}
		return err
	case "report":
		report, err := archive.Report(p)
		if err != nil {
			return err
		}
		return encode.Encode(report)
	default:
		return errors.New("unknown action; use collect, analyze, report or replay")
	}
}

func loadAnalyst(path string) (*catalyst.Analyst, error) {
	entries, err := config.ReadDotEnv(path)
	if err != nil {
		// Keep credential-file diagnostics generic; never echo file contents.
		return nil, errors.New("cannot read catalyst env file")
	}
	values := map[string]string{}
	for _, entry := range entries {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	for _, key := range []string{"TB_CATALYST_BASE_URL", "TB_CATALYST_MODEL", "TB_CATALYST_API_KEY"} {
		if value, present := os.LookupEnv(key); present {
			values[key] = value
		}
	}
	return catalyst.NewAnalyst(values["TB_CATALYST_BASE_URL"], values["TB_CATALYST_MODEL"], values["TB_CATALYST_API_KEY"])
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
