// Live backup-database sync freshness (Phase 18 Step 11).
//
// This is deliberately a SEPARATE function from BackupStatusFor, not a
// forced reuse of it: BackupStatusFor's model assumes a persisted dump
// file that must still exist on disk (DumpFilePresent), which is correct
// for scripts/backup_postgres.sh but wrong for
// scripts/sync_backup_database.sh — that script's dump is a transient
// working file, deleted the moment the sync finishes (see its own
// WORKDIR trap). Forcing it through the file-presence check would
// misreport every successful sync as "artifact missing". The manifest
// itself (row-count-verified, non-zero exit code checked) is the
// evidence of success for this mechanism, not a lingering file.
package integrity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SyncManifest mirrors what scripts/sync_backup_database.sh writes.
type SyncManifest struct {
	SyncID            string `json:"syncId"`
	PrimaryHost       string `json:"primaryHost"`
	PrimaryPort       string `json:"primaryPort"`
	BackupHost        string `json:"backupHost"`
	BackupPort        string `json:"backupPort"`
	DumpSizeBytes     int64  `json:"dumpSizeBytes"`
	DurationSeconds   int64  `json:"durationSeconds"`
	RestoreExitCode   int    `json:"restoreExitCode"`
	RowCountMismatches int   `json:"rowCountMismatches"`
	Status            string `json:"status"`
}

const (
	SyncHealthOK      = "OK"
	SyncHealthStale   = "STALE"
	SyncHealthMissing = "NO_SYNC_FOUND"
	SyncHealthFailed  = "LAST_SYNC_FAILED"
	SyncHealthNotConfigured = "NOT_CONFIGURED"
)

type SyncStatus struct {
	Health string `json:"health"`

	Directory string `json:"directory"`
	SyncCount int    `json:"syncCount"`

	LastSyncID          string     `json:"lastSyncId,omitempty"`
	LastSyncAt          *time.Time `json:"lastSyncAt,omitempty"`
	LastSyncAgeMinutes  *float64   `json:"lastSyncAgeMinutes,omitempty"`
	LastDurationSeconds int64      `json:"lastDurationSeconds,omitempty"`
	LastRowMismatches   int        `json:"lastRowCountMismatches,omitempty"`
	LastStatus          string     `json:"lastStatus,omitempty"`

	MaxAgeMinutes float64 `json:"maxAgeMinutes"`

	Findings []string `json:"findings,omitempty"`
	Note     string   `json:"note"`
}

// SyncStatusFor answers "when was the last successful synchronization,
// and how stale is it?" (Phase 18 Step 11/12). Reads every manifest in
// dir, reports the newest.
func SyncStatusFor(dir string, maxAgeMinutes float64) SyncStatus {
	if maxAgeMinutes <= 0 {
		maxAgeMinutes = 90 // default: sync durations observed at 56-176s, so 90 minutes tolerates a missed hourly run without false-alarming
	}
	s := SyncStatus{
		Directory: dir, MaxAgeMinutes: maxAgeMinutes,
		Note: "Reports the backup-database synchronization mechanism (scripts/sync_backup_database.sh), a separate mechanism from the file-based backup (scripts/backup_postgres.sh, see /admin/ops/backup-status). A healthy sync status means the standing PostgreSQL instance on the configured backup port was verified row-count-consistent with the primary as of the last run — not that a file exists.",
	}
	if dir == "" {
		s.Health = SyncHealthNotConfigured
		s.Findings = append(s.Findings, "No sync manifest directory configured (SYNC_MANIFEST_DIR unset).")
		return s
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		s.Health = SyncHealthMissing
		s.Findings = append(s.Findings, "Sync manifest directory is unreadable or does not exist: "+err.Error())
		return s
	}

	var manifests []SyncManifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".manifest.json") || !strings.HasPrefix(e.Name(), "sync_") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m SyncManifest
		if json.Unmarshal(raw, &m) != nil || m.SyncID == "" {
			continue
		}
		manifests = append(manifests, m)
	}

	s.SyncCount = len(manifests)
	if len(manifests) == 0 {
		s.Health = SyncHealthMissing
		s.Findings = append(s.Findings, "No sync manifests found — synchronization has never run, or writes elsewhere.")
		return s
	}

	sort.Slice(manifests, func(i, j int) bool { return manifests[i].SyncID > manifests[j].SyncID })
	last := manifests[0]

	s.LastSyncID = last.SyncID
	s.LastDurationSeconds = last.DurationSeconds
	s.LastRowMismatches = last.RowCountMismatches
	s.LastStatus = last.Status

	// syncId is "sync_YYYYMMDDTHHMMSSZ".
	if t, err := time.Parse("sync_20060102T150405Z", last.SyncID); err == nil {
		tt := t.UTC()
		s.LastSyncAt = &tt
		age := time.Since(tt).Minutes()
		s.LastSyncAgeMinutes = &age
	}

	s.Health = SyncHealthOK
	if last.Status != "SUCCESS" || last.RowCountMismatches != 0 || last.RestoreExitCode != 0 {
		s.Health = SyncHealthFailed
		s.Findings = append(s.Findings, "Most recent synchronization did not complete cleanly (status/mismatches/exit code).")
	}
	if s.LastSyncAgeMinutes != nil && *s.LastSyncAgeMinutes > maxAgeMinutes {
		s.Health = SyncHealthStale
		s.Findings = append(s.Findings, "Most recent synchronization is older than the configured threshold — nothing is currently scheduling it (this is the expected state until an external scheduler is configured; see scripts/BACKUP_DATABASE_SETUP.md).")
	}

	return s
}
