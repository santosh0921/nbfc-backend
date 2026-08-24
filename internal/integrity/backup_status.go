// Backup monitoring (Production Infrastructure Readiness, Part 7).
//
// The backup script writes a manifest per run. Without something reading
// those manifests, "did last night's backup succeed?" is answerable only
// by an operator listing a directory — which is how backups silently
// stop working for months.
//
// This reads the manifests and exposes the state an alert should key on.
// It is READ-ONLY: it never creates, deletes, or rotates a backup.
//
// SCOPE LIMIT, stated honestly: this monitors a LOCAL backup directory.
// It cannot see off-host copies, because no off-host replication exists
// in this deployment. A green status here means "a local dump was
// produced", NOT "a recoverable off-site backup exists". The response
// carries that caveat so it cannot be misread.
package integrity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupManifest mirrors what scripts/backup_postgres.sh writes.
type BackupManifest struct {
	BackupID        string `json:"backupId"`
	Database        string `json:"database"`
	Host            string `json:"host"`
	Port            string `json:"port"`
	StartedAtUTC    string `json:"startedAtUtc"`
	DurationSeconds int64  `json:"durationSeconds"`
	Mechanism       string `json:"mechanism"`
	SizeBytes       int64  `json:"sizeBytes"`
	SHA256          string `json:"sha256"`
	RetentionDays   int    `json:"retentionDays"`
	Status          string `json:"status"`
}

const (
	BackupHealthOK       = "OK"
	BackupHealthStale    = "STALE"
	BackupHealthMissing  = "NO_BACKUP_FOUND"
	BackupHealthSuspect  = "SUSPECT_SIZE"
	BackupHealthNotConfigured = "NOT_CONFIGURED"
)

type BackupStatus struct {
	Health string `json:"health"`

	Directory  string `json:"directory"`
	BackupCount int   `json:"backupCount"`

	LastBackupID     string     `json:"lastBackupId,omitempty"`
	LastBackupAt     *time.Time `json:"lastBackupAt,omitempty"`
	LastBackupAgeHours *float64 `json:"lastBackupAgeHours,omitempty"`
	LastSizeBytes    int64      `json:"lastSizeBytes,omitempty"`
	LastDurationSeconds int64   `json:"lastDurationSeconds,omitempty"`
	LastChecksum     string     `json:"lastChecksum,omitempty"`
	DumpFilePresent  bool       `json:"dumpFilePresent"`

	// Thresholds are reported so an alert rule and this endpoint cannot
	// disagree about what "stale" means.
	MaxAgeHours       float64 `json:"maxAgeHours"`
	MinExpectedBytes  int64   `json:"minExpectedBytes"`

	Findings []string `json:"findings,omitempty"`
	Caveat   string   `json:"caveat"`
}

// BackupStatusFor inspects a backup directory.
//
// maxAgeHours and minExpectedBytes are the alert thresholds. Defaults
// (26h / 1 MiB) are deliberate: 26h tolerates a daily backup running a
// couple of hours late without crying wolf, and a dump smaller than
// 1 MiB for a database of this shape indicates a truncated or failed
// dump rather than a small database.
func BackupStatusFor(dir string, maxAgeHours float64, minExpectedBytes int64) BackupStatus {
	if maxAgeHours <= 0 {
		maxAgeHours = 26
	}
	if minExpectedBytes <= 0 {
		minExpectedBytes = 1 << 20
	}

	s := BackupStatus{
		Directory: dir, MaxAgeHours: maxAgeHours, MinExpectedBytes: minExpectedBytes,
		Caveat: "This monitors a LOCAL backup directory only. It cannot verify off-host replication, encryption at rest, or that the backup is restorable. A healthy status here means a dump file was produced locally — not that a recoverable off-site backup exists.",
	}

	if dir == "" {
		s.Health = BackupHealthNotConfigured
		s.Findings = append(s.Findings, "No backup directory configured (BACKUP_DESTINATION unset). Backups are not being monitored.")
		return s
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		s.Health = BackupHealthMissing
		s.Findings = append(s.Findings, "Backup directory is unreadable or does not exist: "+err.Error())
		return s
	}

	var manifests []BackupManifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".manifest.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m BackupManifest
		if json.Unmarshal(raw, &m) != nil || m.BackupID == "" {
			s.Findings = append(s.Findings, "Unparseable manifest: "+e.Name())
			continue
		}
		manifests = append(manifests, m)
	}

	s.BackupCount = len(manifests)
	if len(manifests) == 0 {
		s.Health = BackupHealthMissing
		s.Findings = append(s.Findings, "No backup manifests found. Either no backup has ever run, or it is writing elsewhere.")
		return s
	}

	// Newest first by the UTC stamp embedded in the id.
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].StartedAtUTC > manifests[j].StartedAtUTC
	})
	last := manifests[0]

	s.LastBackupID = last.BackupID
	s.LastSizeBytes = last.SizeBytes
	s.LastDurationSeconds = last.DurationSeconds
	s.LastChecksum = last.SHA256

	if t, err := time.Parse("20060102T150405Z", last.StartedAtUTC); err == nil {
		tt := t.UTC()
		s.LastBackupAt = &tt
		age := time.Since(tt).Hours()
		s.LastBackupAgeHours = &age
	}

	// The dump the manifest describes must actually still be present —
	// a manifest without its dump is a backup that no longer exists.
	if _, err := os.Stat(filepath.Join(dir, last.BackupID+".dump")); err == nil {
		s.DumpFilePresent = true
	} else {
		s.Findings = append(s.Findings, "The most recent manifest has no corresponding .dump file — the backup artifact is missing.")
	}

	s.Health = BackupHealthOK
	if last.Status != "SUCCESS" {
		s.Health = BackupHealthMissing
		s.Findings = append(s.Findings, "Most recent backup did not report SUCCESS (status: "+last.Status+").")
	}
	if last.SizeBytes < minExpectedBytes {
		s.Health = BackupHealthSuspect
		s.Findings = append(s.Findings, "Most recent backup is unexpectedly small — likely truncated or failed mid-write.")
	}
	if s.LastBackupAgeHours != nil && *s.LastBackupAgeHours > maxAgeHours {
		s.Health = BackupHealthStale
		s.Findings = append(s.Findings, "Most recent backup is older than the configured threshold — scheduled backups may have stopped running.")
	}
	if !s.DumpFilePresent && s.Health == BackupHealthOK {
		s.Health = BackupHealthMissing
	}

	return s
}
