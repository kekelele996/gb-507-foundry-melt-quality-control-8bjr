package config

import "testing"

func TestLoadRejectsUnknownDatabaseDriver(t *testing.T) {
	t.Setenv("DATABASE_DRIVER", "unknown")
	if _, err := Load(); err == nil {
		t.Fatal("expected unsupported database driver to fail")
	}
}

func TestLoadAcceptsSQLiteForLocalSmoke(t *testing.T) {
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", ":memory:")
	t.Setenv("JWT_SECRET", "this-is-long-enough-for-tests")
	if _, err := Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}
}

func TestProductionRejectsPlaceholderSigningSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", ":memory:")
	t.Setenv("JWT_SECRET", "replace-this-production-secret-value")
	if _, err := Load(); err == nil {
		t.Fatal("expected production placeholder signing secret to fail")
	}
}

func TestProductionAcceptsStrongSigningSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", ":memory:")
	t.Setenv("JWT_SECRET", "7b6a7bf0a64f456caa2b1fb1b90f0d282e49ddd394d847bab5468972d744e4dc")
	if _, err := Load(); err != nil {
		t.Fatalf("load production config: %v", err)
	}
}
