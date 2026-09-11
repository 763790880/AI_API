package migrations

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// These migrations have already run in production. Even changing a brand
// name in a SQL comment prevents startup because it changes the checksum.
func TestAppliedMigrationChecksums(t *testing.T) {
	want := map[string]string{
		"001_init.sql":                   "9ba0369779484625edcea7a7d1d4582397e31546db9149b05004990a3f16c630",
		"002_account_type_migration.sql": "aad3816e44f58ff007ea4df8092aae580f3f85180314c1deb1b1054b20892bbf",
		"003_subscription.sql":           "4642fcb1ccd7954b1d3eef8f795cfba2ce21431257346cc5a7568cde61a60b13",
	}
	for name, checksum := range want {
		t.Run(name, func(t *testing.T) {
			content, err := FS.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			// Git can use CRLF on Windows; release archives use LF.
			normalized := strings.TrimSpace(strings.ReplaceAll(string(content), "\r\n", "\n"))
			if got := fmt.Sprintf("%x", sha256.Sum256([]byte(normalized))); got != checksum {
				t.Fatalf("applied migration changed: checksum=%s, want=%s; restore the original file", got, checksum)
			}
		})
	}
}
