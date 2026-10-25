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

// OIDC Providers 只来自 YAML 列表：默认必须为空（SSO 关闭）。
// TODO(oidc-2): YAML 列表解析断言随流程实现一起补（fixture 在 testdata/oidc/）。
func TestOIDCProvidersDefaultDisabled(t *testing.T) {
	t.Parallel()

	if providers := newViper().GetStringSlice("auth.oidc.providers"); len(providers) != 0 {
		t.Fatalf("auth.oidc.providers default = %v, want empty", providers)
	}
}
