package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

func TestHelpOutputGolden(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
		file string
	}{
		{name: "top-level", args: []string{"help"}, file: "help.golden"},
		{name: "pipeline", args: []string{"help", "pipeline"}, file: "help_pipeline.golden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			if exitCode := run(tc.args, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("expected success, got %d: %s", exitCode, stderr.String())
			}

			assertGoldenFile(t, filepath.Join("testdata", tc.file), stdout.Bytes())
		})
	}
}

func assertGoldenFile(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\nrefresh with: go test ./internal/cli -run TestHelpOutputGolden -args -update", path)
	}
}
