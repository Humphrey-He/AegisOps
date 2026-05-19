package config

import "testing"

func TestNewViperDefaultsToPostgres(t *testing.T) {
	t.Parallel()

	v := newViper()

	if driver := v.GetString("database.driver"); driver != "postgres" {
		t.Fatalf("database.driver default = %q, want %q", driver, "postgres")
	}

	wantDSN := "postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable"
	if dsn := v.GetString("database.dsn"); dsn != wantDSN {
		t.Fatalf("database.dsn default = %q, want %q", dsn, wantDSN)
	}
}
