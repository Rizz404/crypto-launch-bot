// launch-model-eval sends labelled synthetic Plan 4 candidates through the production
// analyzer and validator. Every request also takes a slot from the shared Plan 4 daily
// budget, so a full run spreads over several WIB days; rerunning resumes where it stopped.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/catalyst"
	"github.com/rizz404/crypto-launch-bot/internal/config"
	"github.com/rizz404/crypto-launch-bot/internal/launch"
)

type result struct {
	Case     string          `json:"case"`
	Kind     string          `json:"kind"`
	Covers   string          `json:"covers"`
	Want     []string        `json:"want"`
	Model    string          `json:"requested_model"`
	Passed   bool            `json:"passed"`
	Failure  string          `json:"failure,omitempty"`
	Review   launch.AIReview `json:"review"`
	Duration float64         `json:"seconds"`
}

func score(e evalCase, r launch.AIReview, decoyIDs []string) (bool, string) {
	if !r.Valid {
		return false, "invalid: " + r.Error
	}
	if !slices.Contains(e.Want, r.Decision.Action) {
		return false, "action " + r.Decision.Action + " not in " + strings.Join(e.Want, "/")
	}
	if e.Decoy && r.Decision.Action == "PROPOSE_ENTRY" {
		for _, c := range r.Decision.Citations {
			if slices.Contains(decoyIDs, c.EvidenceID) {
				return false, "entry supported by unrelated decoy evidence"
			}
		}
	}
	return true, ""
}

// decoys are the case's extra evidence: every ID except the snapshot, which stage
// writes first. Seal sorts IDs, so the snapshot is found by content, not position.
func decoys(c launch.Candidate, packet launch.AnalysisPacket) []string {
	var out []string
	for _, ev := range packet.Evidence {
		if !strings.Contains(ev.Text, "Source snapshot for "+c.ExternalID) {
			out = append(out, ev.ID)
		}
	}
	return out
}

func loadAnalyst(path, model string) (*catalyst.Analyst, error) {
	entries, err := config.ReadDotEnv(path)
	if err != nil {
		return nil, errors.New("cannot read launch AI environment")
	}
	env := map[string]string{}
	for _, entry := range entries {
		k, v, _ := strings.Cut(entry, "=")
		env[k] = v
	}
	get := func(suffix string) string {
		if v := env["TB_LAUNCH_"+suffix]; v != "" {
			return v
		}
		return env["TB_CATALYST_"+suffix]
	}
	if model == "" {
		model = get("MODEL")
	}
	return catalyst.NewAnalyst(get("BASE_URL"), model, get("API_KEY"))
}

func run(ctx context.Context) error {
	fs := flag.NewFlagSet("launch-model-eval", flag.ContinueOnError)
	envPath := fs.String("env-file", ".env.launch", "private gateway environment file")
	modelText := fs.String("models", "", "comma-separated routes, at most four; default TB_LAUNCH_MODEL")
	budgetDir := fs.String("budget-archive", "", "the Plan 4 research archive whose daily AI budget is shared (required)")
	outDir := fs.String("out", "var/launch-eval", "result directory; existing results are kept and skipped")
	only := fs.String("cases", "", "optional comma-separated case IDs")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *budgetDir == "" || fs.NArg() != 0 {
		return errors.New("-budget-archive is required so evaluation cannot bypass the daily AI budget")
	}
	models := []string{""}
	if *modelText != "" {
		models = strings.Split(*modelText, ",")
	}
	if len(models) > 4 {
		return errors.New("at most four models")
	}
	budget := catalyst.Archive{Dir: *budgetDir}
	selected := cases()
	if *only != "" {
		ids := strings.Split(*only, ",")
		selected = slices.DeleteFunc(selected, func(e evalCase) bool { return !slices.Contains(ids, e.ID) })
	}
	var all []result
	for _, m := range models {
		analyst, err := loadAnalyst(*envPath, strings.TrimSpace(m))
		if err != nil {
			return err
		}
		dir := filepath.Join(*outDir, strings.NewReplacer("/", "_", ":", "_").Replace(analyst.Model))
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		for n, e := range cases() {
			if !slices.ContainsFunc(selected, func(s evalCase) bool { return s.ID == e.ID }) {
				continue
			}
			path := filepath.Join(dir, e.ID+".json")
			if old, err := os.ReadFile(path); err == nil {
				var r result
				if json.Unmarshal(old, &r) == nil {
					all = append(all, r)
					continue
				}
			}
			now := time.Now().UTC()
			archive := catalyst.Archive{Dir: filepath.Join(dir, "archives", e.ID+"-"+now.Format("20060102T150405Z")), MaxBytes: 16 << 20}
			c, state, err := e.stage(archive, n, now)
			if err != nil {
				return fmt.Errorf("%s: stage: %w", e.ID, err)
			}
			packet, err := launch.BuildAnalysisPacket(archive, c.ID, launch.ApprovedPaperPolicy(), state, now)
			if err != nil {
				return fmt.Errorf("%s: packet: %w", e.ID, err)
			}
			if packet.Verification.Status != e.WantStatus {
				return fmt.Errorf("%s: fixture verified as %s, want %s (no slot spent)", e.ID, packet.Verification.Status, e.WantStatus)
			}
			if err = budget.ReserveResearchInference(now); err != nil {
				fmt.Println("stopping:", err)
				return summarize(all)
			}
			start := time.Now()
			review, _ := launch.Analyze(ctx, archive, analyst.Model, analyst, packet)
			r := result{Case: e.ID, Kind: e.Kind, Covers: e.Covers, Want: e.Want, Model: analyst.Model, Review: review, Duration: time.Since(start).Seconds()}
			r.Passed, r.Failure = score(e, review, decoys(c, packet))
			b, _ := json.MarshalIndent(r, "", "  ")
			if err = os.WriteFile(path, b, 0600); err != nil {
				return err
			}
			fmt.Printf("%-26s %-14s passed=%-5v action=%-13s %5.1fs %s\n", e.ID, review.ServedModel, r.Passed, review.Decision.Action, r.Duration, r.Failure)
			all = append(all, r)
		}
	}
	return summarize(all)
}

func summarize(all []result) error {
	type tally struct{ passed, done int }
	byKind := map[string]*tally{}
	for _, r := range all {
		key := r.Model + " " + r.Kind
		if byKind[key] == nil {
			byKind[key] = &tally{}
		}
		byKind[key].done++
		if r.Passed {
			byKind[key].passed++
		}
	}
	keys := make([]string, 0, len(byKind))
	for k := range byKind {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Printf("%-45s %d/%d passed (of 10)\n", k, byKind[k].passed, byKind[k].done)
	}
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
