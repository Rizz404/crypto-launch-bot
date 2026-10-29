// launch performs research discovery only. It never reads trading credentials.
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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/launch"
)

func run(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet("launch", flag.ContinueOnError)
	action := f.String("action", "report", "report, collect, analyze, replay-paper (synthetic offline) or paper-report")
	kind := f.String("source", "all", "all, presale, dex or cex")
	dir := f.String("archive", "var/launch", "isolated research archive, never a financial ledger")
	spider := f.String("spider-binary", "spider", "pinned Spider CLI for presale website collection")
	limit := f.Int("limit", 5, "DEX tail/CEX symbols per pass, 1–10; presale uses the 5-page Spider budget")
	candidateID := f.String("candidate", "", "candidate ID from report, required for analyze")
	policyPath := f.String("policy", "configs/launch.policy.json", "approved paper-only analysis policy")
	contextPath := f.String("context", "", "optional explicit paper-context JSON; otherwise initial virtual state")
	envPath := f.String("env-file", ".env.launch", "AI credentials, read only by explicit analyze action")
	fixturePath := f.String("fixture", "", "synthetic paper fixture JSON for replay-paper")
	paperDir := f.String("paper-archive", "var/launch-paper", "isolated synthetic paper journal; never a live wallet")
	runID := f.String("run-id", "", "existing paper run ID, required for paper-report")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *dir == "" {
		return errors.New("archive path is required")
	}
	resolved, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	archive := catalyst.Archive{Dir: resolved, MaxBytes: 256 << 20}
	if *action == "replay-paper" || *action == "paper-report" {
		if *paperDir == "" {
			return errors.New("paper archive path required")
		}
		paperPath, pathErr := filepath.Abs(*paperDir)
		if pathErr != nil {
			return pathErr
		}
		if filepath.Clean(paperPath) == filepath.Clean(resolved) {
			return errors.New("paper journal must be separate from research archive")
		}
		paperArchive := catalyst.Archive{Dir: paperPath, MaxBytes: 256 << 20}
		var state launch.PaperState
		if *action == "replay-paper" {
			if *fixturePath == "" {
				return errors.New("replay-paper requires an explicit synthetic fixture")
			}
			fixtureFile, openErr := os.Open(*fixturePath)
			if openErr != nil {
				return errors.New("cannot open paper fixture")
			}
			var fixture launch.PaperFixture
			err = catalyst.Decode(fixtureFile, &fixture, 16<<20)
			_ = fixtureFile.Close()
			if err != nil {
				return err
			}
			state, err = launch.ReplayPaper(ctx, paperArchive, fixture)
		} else {
			var p launch.PaperPolicy
			if err = readJSON(*policyPath, &p); err != nil {
				return err
			}
			state, err = (launch.PaperStore{Archive: paperArchive, RunID: *runID, Policy: p}).Load()
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(struct {
			State   launch.PaperState   `json:"state"`
			Context launch.PaperContext `json:"context"`
		}{state, state.Context(state.At)})
	}
	var report launch.Report
	switch *action {
	case "report":
		report, err = launch.ReadReport(archive, time.Now().UTC())
	case "collect":
		ctx, cancel := context.WithTimeout(ctx, 12*time.Minute)
		defer cancel()
		report, err = launch.Collect(ctx, archive, *kind, *spider, *limit)
	case "analyze":
		if *candidateID == "" {
			return errors.New("analyze requires a candidate ID from report")
		}
		p := launch.PaperPolicy{}
		if err = readJSON(*policyPath, &p); err != nil {
			return err
		}
		if err = p.Validate(); err != nil {
			return err
		}
		now := time.Now().UTC()
		state := launch.InitialPaperContext(now, p)
		if *contextPath != "" {
			state = launch.PaperContext{}
			if err = readJSON(*contextPath, &state); err != nil {
				return err
			}
		}
		packet, packetErr := launch.BuildAnalysisPacket(archive, *candidateID, p, state, now)
		if packetErr != nil {
			return packetErr
		}
		analyst, loadErr := loadAnalyst(*envPath)
		if loadErr != nil {
			return loadErr
		}
		review, analysisErr := launch.Analyze(ctx, archive, analyst.Model, analyst, packet)
		if err = json.NewEncoder(out).Encode(review); err != nil {
			return err
		}
		return analysisErr
	default:
		return errors.New("unknown action; only research and synthetic paper replay are supported")
	}
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func readJSON(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("cannot open analysis policy/context")
	}
	defer func() { _ = f.Close() }()
	return catalyst.Decode(f, out, 64<<10)
}

func loadAnalyst(path string) (*catalyst.Analyst, error) {
	entries, err := config.ReadDotEnv(path)
	if err != nil {
		return nil, errors.New("cannot read launch AI environment")
	}
	values := map[string]string{}
	for _, entry := range entries {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	for _, prefix := range []string{"TB_CATALYST_", "TB_LAUNCH_"} {
		for _, suffix := range []string{"BASE_URL", "MODEL", "API_KEY"} {
			key := prefix + suffix
			if value, present := os.LookupEnv(key); present {
				values[key] = value
			}
		}
	}
	get := func(suffix string) string {
		if value, present := values["TB_LAUNCH_"+suffix]; present {
			return value
		}
		return values["TB_CATALYST_"+suffix]
	}
	return catalyst.NewAnalyst(get("BASE_URL"), get("MODEL"), get("API_KEY"))
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
