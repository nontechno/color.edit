package incident

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportRedactsSecretsAndUsesPrivatePermissions(t *testing.T) {
	reporter := &Reporter{Dir: t.TempDir(), Backend: "mysql"}
	reference := reporter.Record("connect", errors.New("password=hunter2 oracle://user:pass@host/db user:pw@tcp(host) /home/alice/private"), []byte("token=abc C:\\Users\\alice\\project"), nil)
	if filepath.Base(reference) != reference {
		t.Fatalf("report reference exposes an absolute path: %q", reference)
	}
	contents, err := os.ReadFile(filepath.Join(reporter.Dir, reference))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, secret := range []string{"hunter2", "user:pass", "user:pw", "token=abc", "/home/alice", `C:\Users\alice`} {
		if strings.Contains(text, secret) {
			t.Fatalf("report leaked %q: %s", secret, text)
		}
	}
	info, err := os.Stat(filepath.Join(reporter.Dir, reference))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("report mode is too permissive: %o", info.Mode().Perm())
	}
}
