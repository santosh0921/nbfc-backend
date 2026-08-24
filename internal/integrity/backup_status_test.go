package integrity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/santosh0921/nbfc-backend/internal/integrity"
)

// Backup monitoring (Part 7) tested against BOTH the real backup
// produced by scripts/backup_postgres.sh and synthetic failure cases.

func TestBackupStatusAgainstRealBackup(t *testing.T) {
	dir := os.Getenv("BACKUP_DESTINATION")
	if dir == "" {
		dir = filepath.Join(os.Getenv("USERPROFILE"), "nbfc-backups")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no real backup directory at %s — run scripts/backup_postgres.sh first", dir)
	}
	s := integrity.BackupStatusFor(dir, 0, 0)
	t.Logf("health=%s count=%d id=%s size=%d dur=%ds dumpPresent=%v findings=%v",
		s.Health, s.BackupCount, s.LastBackupID, s.LastSizeBytes,
		s.LastDurationSeconds, s.DumpFilePresent, s.Findings)

	if s.BackupCount == 0 {
		t.Fatal("expected at least one manifest from the real backup run")
	}
	if s.LastChecksum == "" {
		t.Error("backup status must surface the checksum — it is the integrity evidence")
	}
	if !s.DumpFilePresent {
		t.Error("the dump file for the most recent manifest is missing")
	}
	if s.Caveat == "" {
		t.Error("status must state that it monitors only a LOCAL directory")
	}
}

// Each alert condition must be detected, not just the happy path.
func TestBackupStatusDetectsFailureConditions(t *testing.T) {
	t.Run("no directory configured", func(t *testing.T) {
		s := integrity.BackupStatusFor("", 0, 0)
		if s.Health != integrity.BackupHealthNotConfigured {
			t.Fatalf("expected NOT_CONFIGURED, got %s", s.Health)
		}
	})

	t.Run("directory missing", func(t *testing.T) {
		s := integrity.BackupStatusFor(filepath.Join(t.TempDir(), "does-not-exist"), 0, 0)
		if s.Health != integrity.BackupHealthMissing {
			t.Fatalf("expected NO_BACKUP_FOUND, got %s", s.Health)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		s := integrity.BackupStatusFor(t.TempDir(), 0, 0)
		if s.Health != integrity.BackupHealthMissing {
			t.Fatalf("expected NO_BACKUP_FOUND for an empty directory, got %s", s.Health)
		}
	})

	t.Run("stale backup", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "nbfc_db_20200101T000000Z", 50_000_000, "SUCCESS")
		s := integrity.BackupStatusFor(dir, 26, 0)
		if s.Health != integrity.BackupHealthStale {
			t.Fatalf("a 2020 backup must be STALE, got %s", s.Health)
		}
		if s.LastBackupAgeHours == nil || *s.LastBackupAgeHours < 1000 {
			t.Fatal("stale backup must report a large age")
		}
	})

	t.Run("suspiciously small backup", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "nbfc_db_20200101T000000Z", 12, "SUCCESS")
		// Large max-age so STALE does not mask the size finding.
		s := integrity.BackupStatusFor(dir, 1_000_000, 1<<20)
		if s.Health != integrity.BackupHealthSuspect {
			t.Fatalf("a 12-byte backup must be SUSPECT_SIZE, got %s", s.Health)
		}
	})

	t.Run("manifest without dump file", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "nbfc_db_20200101T000000Z", 50_000_000, "SUCCESS")
		os.Remove(filepath.Join(dir, "nbfc_db_20200101T000000Z.dump"))
		s := integrity.BackupStatusFor(dir, 1_000_000, 0)
		if s.DumpFilePresent {
			t.Fatal("must detect that the dump artifact is gone")
		}
		if len(s.Findings) == 0 {
			t.Fatal("a missing dump artifact must produce a finding")
		}
	})
}

func writeManifest(t *testing.T, dir, id string, size int64, status string) {
	t.Helper()
	body := `{"backupId":"` + id + `","database":"nbfc_db","startedAtUtc":"` +
		id[len("nbfc_db_"):] + `","durationSeconds":10,"sizeBytes":` +
		itoa64(size) + `,"sha256":"deadbeef","retentionDays":30,"status":"` + status + `"}`
	if err := os.WriteFile(filepath.Join(dir, id+".manifest.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".dump"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
