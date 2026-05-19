package exporter

import (
	"strings"
	"testing"
)

func TestBackupManifestFilesForPostgres(t *testing.T) {
	t.Parallel()

	service := NewService(nil, Options{
		DBDriver: "postgres",
		DBDSN:    "postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable",
	})

	files := service.backupManifestFiles()
	if len(files) != 1 || files[0] != "postgresql-backup.placeholder.txt" {
		t.Fatalf("backupManifestFiles() = %v, want postgres placeholder", files)
	}

	dbFiles, err := service.databaseFiles()
	if err != nil {
		t.Fatalf("databaseFiles() error = %v", err)
	}
	if _, ok := dbFiles["postgresql-backup.placeholder.txt"]; !ok {
		t.Fatalf("databaseFiles() = %v, want postgres placeholder file", dbFiles)
	}
	if _, ok := dbFiles["backup.db"]; ok {
		t.Fatalf("databaseFiles() unexpectedly contains backup.db in postgres mode")
	}

	guide := service.restoreGuide()
	if !strings.Contains(guide, "pg_dump") {
		t.Fatalf("restoreGuide() = %q, want pg_dump guidance", guide)
	}
}

func TestBackupManifestFilesForSQLite(t *testing.T) {
	t.Parallel()

	service := NewService(nil, Options{
		DBDriver: "sqlite",
		DBDSN:    "data/aegisops.db",
	})

	files := service.backupManifestFiles()
	if len(files) != 1 || files[0] != "backup.db" {
		t.Fatalf("backupManifestFiles() = %v, want backup.db", files)
	}

	guide := service.restoreGuide()
	if !strings.Contains(guide, "`backup.db`") {
		t.Fatalf("restoreGuide() = %q, want backup.db guidance", guide)
	}
}
