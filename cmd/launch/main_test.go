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
}
