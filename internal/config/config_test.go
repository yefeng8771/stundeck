package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDerivesPersistentFilesFromDataDir(t *testing.T) {
	t.Setenv("STUNDECK_DATA_DIR", "/var/lib/stundeck-fpk")
	t.Setenv("STUNDECK_DATABASE", "")
	t.Setenv("STUNDECK_MASTER_KEY_FILE", "")

	loaded := Load()

	if loaded.DataDir != "/var/lib/stundeck-fpk" {
		t.Fatalf("DataDir = %q", loaded.DataDir)
	}
	if loaded.DatabasePath != filepath.Join(loaded.DataDir, "stundeck.db") {
		t.Fatalf("DatabasePath = %q", loaded.DatabasePath)
	}
	if loaded.MasterKeyFile != filepath.Join(loaded.DataDir, "master.key") {
		t.Fatalf("MasterKeyFile = %q", loaded.MasterKeyFile)
	}
}

func TestLoadAllowsPlatformPathOverrides(t *testing.T) {
	t.Setenv("STUNDECK_DATA_DIR", "/var/lib/stundeck")
	t.Setenv("STUNDECK_DATABASE", "/mnt/state/database.sqlite")
	t.Setenv("STUNDECK_MASTER_KEY_FILE", "/mnt/secrets/master.key")
	t.Setenv("STUNDECK_NATMAP_BINARY", "/opt/stundeck/natmap")
	t.Setenv("STUNDECK_NOTIFY_BINARY", "/opt/stundeck/stundeck-notify")

	loaded := Load()

	if loaded.DatabasePath != "/mnt/state/database.sqlite" {
		t.Fatalf("DatabasePath = %q", loaded.DatabasePath)
	}
	if loaded.MasterKeyFile != "/mnt/secrets/master.key" {
		t.Fatalf("MasterKeyFile = %q", loaded.MasterKeyFile)
	}
	if loaded.NatmapBinary != "/opt/stundeck/natmap" {
		t.Fatalf("NatmapBinary = %q", loaded.NatmapBinary)
	}
	if loaded.NotifyBinary != "/opt/stundeck/stundeck-notify" {
		t.Fatalf("NotifyBinary = %q", loaded.NotifyBinary)
	}
}
