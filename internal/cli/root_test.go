package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecuteExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{
			name: "no arguments prints help and succeeds",
			args: nil,
			want: exitOK,
		},
		{
			name: "version succeeds",
			args: []string{"version"},
			want: exitOK,
		},
		{
			name: "explicit help succeeds",
			args: []string{"--help"},
			want: exitOK,
		},
		{
			name: "an unknown command is a usage error",
			args: []string{"bogus"},
			want: exitUsage,
		},
		{
			name: "an unknown flag is a usage error",
			args: []string{"--nosuchflag"},
			want: exitUsage,
		},
		{
			name: "an argument to a command that takes none is a usage error",
			args: []string{"version", "extra"},
			want: exitUsage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer

			if got := execute(tt.args, &out, &errOut); got != tt.want {
				t.Errorf("execute(%q) = %d, want %d\nstdout: %s\nstderr: %s",
					tt.args, got, tt.want, out.String(), errOut.String())
			}
		})
	}
}

func TestBareCommandPrintsHelp(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := execute(nil, &out, &errOut); code != exitOK {
		t.Fatalf("execute(nil) = %d, want %d", code, exitOK)
	}

	got := out.String()
	for _, want := range []string{"upall updates everything on this machine", "Usage:", "version"} {
		if !strings.Contains(got, want) {
			t.Errorf("help output does not mention %q; got:\n%s", want, got)
		}
	}
}

func TestVersionCommandOutput(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := execute([]string{"version"}, &out, &errOut); code != exitOK {
		t.Fatalf("execute([version]) = %d, want %d\nstderr: %s", code, exitOK, errOut.String())
	}

	got := out.String()
	if !strings.HasPrefix(got, "upall ") {
		t.Errorf("version output should start with %q; got:\n%s", "upall ", got)
	}
	// The second line is the toolchain and target, which is what makes a bug
	// report actionable.
	if lines := strings.Split(strings.TrimSpace(got), "\n"); len(lines) != 2 {
		t.Errorf("version output should be two lines, got %d:\n%s", len(lines), got)
	}
	if errOut.Len() != 0 {
		t.Errorf("version wrote to stderr: %s", errOut.String())
	}
}

// TestUsageErrorsExplainThemselves guards against a usage error that exits 2
// while saying nothing about what was wrong.
func TestUsageErrorsExplainThemselves(t *testing.T) {
	var out, errOut bytes.Buffer

	execute([]string{"bogus"}, &out, &errOut)

	combined := out.String() + errOut.String()
	if !strings.Contains(combined, "bogus") {
		t.Errorf("the error should name the unknown command; got:\n%s", combined)
	}
}
