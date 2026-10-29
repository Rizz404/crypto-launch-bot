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
	"syscall"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/rizz404/trading-bot/internal/launch"
)

func run(ctx context.Context, args []string, out io.Writer) error {
	f := flag.NewFlagSet("launch", flag.ContinueOnError)
	action := f.String("action", "report", "report (offline) or collect (bounded public read-only requests)")
	kind := f.String("source", "all", "all, presale, dex or cex")
	dir := f.String("archive", "var/launch", "isolated research archive, never a financial ledger")
	spider := f.String("spider-binary", "spider", "pinned Spider CLI for presale website collection")
	limit := f.Int("limit", 5, "DEX tail/CEX symbols per pass, 1–10; presale uses the 5-page Spider budget")
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
	var report launch.Report
	switch *action {
	case "report":
		report, err = launch.ReadReport(archive, time.Now().UTC())
	case "collect":
		ctx, cancel := context.WithTimeout(ctx, 12*time.Minute)
		defer cancel()
		report, err = launch.Collect(ctx, archive, *kind, *spider, *limit)
	default:
		return errors.New("action must be report or collect; trading is not supported")
	}
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
