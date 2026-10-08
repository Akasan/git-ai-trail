package git

import (
	"strings"
	"testing"
)

func TestParseBlame(t *testing.T) {
	blameOutput := `abc123def0123456789012345678901234567890 1 1 1
author John Doe
	first line
def456abc0123456789012345678901234567890 2 2 1
author Jane Doe
	second line`

	lines := parseBlame(blameOutput)

	if len(lines) != 2 {
		t.Errorf("Expected 2 blame lines, got %d", len(lines))
	}

	if len(lines) > 0 && !strings.HasPrefix(lines[0].Commit, "abc123def") {
		t.Errorf("Expected commit starting with abc123def, got %s", lines[0].Commit)
	}

	if len(lines) > 0 && lines[0].Content != "first line" {
		t.Errorf("Expected content 'first line', got '%s'", lines[0].Content)
	}
}

func TestParseUnifiedDiff(t *testing.T) {
	diff := `@@ -1,3 +1,4 @@
 line1
+line2
 line3
+line4`

	added, _, err := parseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("parseUnifiedDiff failed: %v", err)
	}

	if len(added) != 2 {
		t.Errorf("Expected 2 added lines, got %d", len(added))
	}
}
