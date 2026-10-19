package store

import (
	"testing"
)

func TestRunHistoryAppendListClear(t *testing.T) {
	dir := t.TempDir()

	if runs, err := ListRuns(dir, 10); err != nil || len(runs) != 0 {
		t.Fatalf("empty history: runs=%v err=%v", runs, err)
	}

	for i := 1; i <= 5; i++ {
		rec := RunRecord{StartedAt: "t", Profile: "p", Mode: "once", StopReason: "done", Clicks: int64(i)}
		if err := AppendRun(dir, rec, 3); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := ListRuns(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("kept %d runs, want 3 (cap)", len(runs))
	}
	// Newest first.
	if runs[0].Clicks != 5 || runs[2].Clicks != 3 {
		t.Fatalf("order wrong: %+v", runs)
	}

	if err := ClearRuns(dir); err != nil {
		t.Fatal(err)
	}
	if runs, err := ListRuns(dir, 10); err != nil || len(runs) != 0 {
		t.Fatalf("after clear: runs=%v err=%v", runs, err)
	}
	// Clearing an absent file is not an error.
	if err := ClearRuns(dir); err != nil {
		t.Fatalf("second clear: %v", err)
	}
}
