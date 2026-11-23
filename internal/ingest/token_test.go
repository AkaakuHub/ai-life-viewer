package ingest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAuthCreatesPrivateReusableTokenAndHonorsEnvironment(t *testing.T) {
	t.Setenv(TokenEnv, "")
	path := filepath.Join(t.TempDir(), "token")
	generated, err := LoadAuth(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected token file permissions: %o", info.Mode().Perm())
	}
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header := http.Header{"Authorization": {"Bearer " + strings.TrimSpace(string(value))}}
	reloaded, err := LoadAuth(path)
	if err != nil || !generated.Valid(header) || !reloaded.Valid(header) {
		t.Fatalf("generated token unavailable after reopening: %v", err)
	}
	t.Setenv(TokenEnv, "configured")
	override, err := LoadAuth(path)
	if err != nil || override.Valid(header) || !override.Valid(http.Header{"Authorization": {"Bearer configured"}}) {
		t.Fatalf("environment override failed: %v", err)
	}
}
