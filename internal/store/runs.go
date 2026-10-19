package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RunsFile is the append-only run history, kept in the profiles folder next to
// the profiles (DESIGN §5.9). One JSON object per line.
const RunsFile = "runs.jsonl"

// RunRecord is one completed run.
type RunRecord struct {
	StartedAt  string `json:"startedAt"` // RFC3339, UTC
	Profile    string `json:"profile"`
	Mode       string `json:"mode"` // loop mode
	StopReason string `json:"stopReason"`
	Clicks     int64  `json:"clicks"`
	ElapsedMs  int64  `json:"elapsedMs"`
	Error      string `json:"error,omitempty"`
}

// RunsPath returns the run-history file for a profiles dir.
func RunsPath(dir string) string { return filepath.Join(dir, RunsFile) }

// AppendRun appends rec and trims the file to at most limit lines, keeping the
// newest. limit <= 0 falls back to DefaultRunHistoryLimit.
func AppendRun(dir string, rec RunRecord, limit int) error {
	if limit <= 0 {
		limit = DefaultRunHistoryLimit
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(RunsPath(dir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return trimRuns(dir, limit)
}

func trimRuns(dir string, limit int) error {
	data, err := os.ReadFile(RunsPath(dir))
	if err != nil {
		return err
	}
	lines := splitLines(data)
	if len(lines) <= limit {
		return nil
	}
	kept := lines[len(lines)-limit:]
	return WriteFileAtomic(RunsPath(dir), []byte(strings.Join(kept, "\n")+"\n"))
}

// ListRuns returns up to limit records, newest first. A missing file is empty.
func ListRuns(dir string, limit int) ([]RunRecord, error) {
	if limit <= 0 {
		limit = DefaultRunHistoryLimit
	}
	data, err := os.ReadFile(RunsPath(dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := splitLines(data)
	out := make([]RunRecord, 0, limit)
	for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
		var rec RunRecord
		if json.Unmarshal([]byte(lines[i]), &rec) != nil {
			continue // skip a corrupt line rather than fail the whole list
		}
		out = append(out, rec)
	}
	return out, nil
}

// ClearRuns deletes the run history.
func ClearRuns(dir string) error {
	err := os.Remove(RunsPath(dir))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func splitLines(data []byte) []string {
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	out := raw[:0]
	for _, l := range raw {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
