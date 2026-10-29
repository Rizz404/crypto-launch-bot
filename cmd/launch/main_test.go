package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestOfflineReportAndNoTradingAction(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-archive", t.TempDir()}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"research_only": true`) {
		t.Fatal("report must be research only")
	}
	for _, action := range []string{"buy", "run", "claim", "sell"} {
		if err := run(context.Background(), []string{"-action", action, "-archive", t.TempDir()}, &out); err == nil {
			t.Fatalf("accepted %s", action)
		}
	}
	if err := run(context.Background(), []string{"-action", "analyze", "-archive", t.TempDir()}, &out); err == nil {
		t.Fatal("analysis without candidate accepted")
	}
}

func TestPaperReplayRequiresFixtureAndSeparateArchive(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()
	if err := run(context.Background(), []string{"-action", "replay-paper", "-archive", dir, "-paper-archive", dir}, &out); err == nil {
		t.Fatal("paper journal shared the research archive")
	}
	if err := run(context.Background(), []string{"-action", "replay-paper", "-paper-archive", t.TempDir()}, &out); err == nil {
		t.Fatal("paper replay without explicit fixture accepted")
	}
	if err := run(context.Background(), []string{"-action", "paper-report", "-paper-archive", t.TempDir()}, &out); err == nil {
		t.Fatal("paper report without run ID accepted")
	}
}
