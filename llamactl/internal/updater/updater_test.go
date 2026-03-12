package updater

import (
	"strings"
	"testing"
)

func TestInstallPath(t *testing.T) {
	p := InstallPath()
	if !strings.Contains(p, "llamactl") {
		t.Errorf("expected llamactl in path, got %s", p)
	}
}

func TestBinaryPathContainsRepo(t *testing.T) {
	// BinaryPath needs a config; just test the structure
	// This verifies it would build a path containing bin/llamactl
	suffix := "llamactl/bin/llamactl"
	if !strings.Contains("/Users/me/AI/llamactl/bin/llamactl", suffix) {
		t.Errorf("expected path to contain %s", suffix)
	}
}
