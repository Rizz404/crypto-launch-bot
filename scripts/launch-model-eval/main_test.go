package main

import (
	"slices"
	"testing"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/catalyst"
	"github.com/rizz404/crypto-launch-bot/internal/launch"
)

// Every fixture must verify as labelled before any model sees it, so a run
// never spends AI slots on a case whose ground truth is wrong.
func TestCasesBuildIntoReadablePackets(t *testing.T) {
	now := time.Now().UTC()
	perKind := map[string]int{}
	seen := map[string]bool{}
	for n, e := range cases() {
		if seen[e.ID] {
			t.Fatalf("duplicate case %s", e.ID)
		}
		seen[e.ID] = true
		perKind[e.Kind]++
		archive := catalyst.Archive{Dir: t.TempDir()}
		c, state, err := e.stage(archive, n, now)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		packet, err := launch.BuildAnalysisPacket(archive, c.ID, launch.ApprovedPaperPolicy(), state, now)
		if err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
		if packet.Verification.Status != e.WantStatus {
			t.Errorf("%s: verified %s, want %s: %v", e.ID, packet.Verification.Status, e.WantStatus, packet.Verification.Reasons)
		}
		if packet.AssetKey == "" || len(packet.Evidence) != 1+len(e.Evidence) {
			t.Errorf("%s: asset %q, evidence %d", e.ID, packet.AssetKey, len(packet.Evidence))
		}
		// An entry or contribution is only an acceptable label where policy could allow it.
		if (slices.Contains(e.Want, "PROPOSE_ENTRY") || slices.Contains(e.Want, "PROPOSE_CONTRIBUTE")) && e.WantStatus != "accepted_for_research" {
			t.Errorf("%s: proposal labelled acceptable on a %s candidate", e.ID, e.WantStatus)
		}
		if e.position && len(state.Positions) != 1 {
			t.Errorf("%s: missing paper exposure", e.ID)
		}
		if got := decoys(c, packet); len(got) != len(e.Evidence) {
			t.Errorf("%s: %d decoys, want %d", e.ID, len(got), len(e.Evidence))
		}
	}
	for _, kind := range []string{"curve", "dex", "cex", "presale"} {
		if perKind[kind] != 10 {
			t.Errorf("%s has %d cases, want 10", kind, perKind[kind])
		}
	}
}

func TestScoreRequiresValidAllowedActionAndNoDecoyEntry(t *testing.T) {
	e := evalCase{Want: []string{"WATCH", "PROPOSE_ENTRY"}, Decoy: true}
	ok := launch.AIReview{Valid: true, Decision: launch.Decision{Action: "WATCH"}}
	if passed, _ := score(e, ok, nil); !passed {
		t.Fatal("allowed action failed")
	}
	if passed, _ := score(e, launch.AIReview{Valid: false, Decision: launch.Decision{Action: "WATCH"}}, nil); passed {
		t.Fatal("invalid review passed")
	}
	entry := launch.AIReview{Valid: true, Decision: launch.Decision{Action: "PROPOSE_ENTRY", Citations: []catalyst.Citation{{EvidenceID: "decoy"}}}}
	if passed, _ := score(e, entry, []string{"decoy"}); passed {
		t.Fatal("entry citing decoy evidence passed")
	}
	if passed, _ := score(evalCase{Want: []string{"REJECT"}}, ok, nil); passed {
		t.Fatal("disallowed action passed")
	}
}
