package exec

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// recorder is a slog.Handler that keeps the records it was given, so a test can
// assert on what was logged rather than on text that happened to be formatted.
type recorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recorder) Enabled(context.Context, slog.Level) bool { return true }

func (h *recorder) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recorder) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recorder) WithGroup(string) slog.Handler      { return h }

// only returns the one record expected, failing if there is any other number.
func (h *recorder) only(t *testing.T) slog.Record {
	t.Helper()

	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.records) != 1 {
		t.Fatalf("logged %d records, want exactly 1 per command", len(h.records))
	}
	return h.records[0]
}

// attrs flattens a record's attributes into a map keyed by name.
func attrs(r slog.Record) map[string]slog.Value {
	found := make(map[string]slog.Value, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		found[a.Key] = a.Value
		return true
	})
	return found
}

func TestRunLogsTheCommandAtDebug(t *testing.T) {
	h := &recorder{}
	r := testRunnerWithLogger(slog.New(h))

	if _, err := r.Run(t.Context(), helperCommand(t, "echo", "hello")); err != nil {
		t.Fatalf("Run() returned %v, want no error", err)
	}

	record := h.only(t)
	if record.Level != slog.LevelDebug {
		t.Errorf("logged at %s, want %s; a command per line at info would drown a run", record.Level, slog.LevelDebug)
	}

	got := attrs(record)
	for _, want := range []string{"argv", "duration", "exit_code"} {
		if _, found := got[want]; !found {
			t.Errorf("the record has no %q attribute, which the M3 criterion names", want)
		}
	}
	if code := got["exit_code"].Int64(); code != 0 {
		t.Errorf("exit_code = %d, want 0", code)
	}
	if d := got["duration"].Duration(); d <= 0 {
		t.Errorf("duration = %s, want the time the command took", d)
	}
}

func TestRunLogsAFailingCommand(t *testing.T) {
	h := &recorder{}
	r := testRunnerWithLogger(slog.New(h))

	if _, err := r.Run(t.Context(), helperCommand(t, "exit", "3", "it broke")); err == nil {
		t.Fatal("Run() returned no error, want one")
	}

	got := attrs(h.only(t))
	if code := got["exit_code"].Int64(); code != 3 {
		t.Errorf("exit_code = %d, want 3", code)
	}
	if _, found := got["error"]; !found {
		t.Error("a failed command logged no error attribute")
	}
}

// TestRunNeverLogsTheEnvironmentOrTheOutput is the "no secrets and no full
// environment" half of the criterion, and it is the reason this file asserts on
// records rather than eyeballing output.
//
// The environment routinely holds credentials, and package manager output
// carries repository URLs with credentials embedded. At debug level either ends
// up pasted into a bug report.
func TestRunNeverLogsTheEnvironmentOrTheOutput(t *testing.T) {
	const (
		// The command is asked to print this variable, so the value is both an
		// environment value and the captured output. One assertion then covers
		// both leaks, and neither string appears in the argv, which is logged
		// in full on purpose.
		secretValue = "hunter2-do-not-log-this"

		// Never named on the command line, so finding it could only mean the
		// keys of the overlay were logged.
		secretKey = "UPALL_TEST_UNMENTIONED_KEY"
	)

	h := &recorder{}
	r := testRunnerWithLogger(slog.New(h))

	cmd := helperCommand(t, "env", "UPALL_TEST_TOKEN")
	cmd.Env = helperEnviron("UPALL_TEST_TOKEN="+secretValue, secretKey+"=also-secret")

	out, err := r.Run(t.Context(), cmd)
	if err != nil {
		t.Fatalf("Run() returned %v, want no error", err)
	}
	// Confirm the command really did emit the secret, so that the assertions
	// below are checking a filter rather than an empty stream.
	if got := string(out.Stdout); got != secretValue {
		t.Fatalf("the command wrote %q, want the secret; the test would prove nothing", got)
	}

	record := h.only(t)

	var logged strings.Builder
	logged.WriteString(record.Message)
	record.Attrs(func(a slog.Attr) bool {
		logged.WriteString(" ")
		logged.WriteString(a.Key)
		logged.WriteString("=")
		logged.WriteString(a.Value.String())
		return true
	})

	if strings.Contains(logged.String(), secretValue) {
		t.Errorf("an environment value, and the output that echoed it, reached the log: %s", logged.String())
	}
	if strings.Contains(logged.String(), secretKey) {
		t.Error("an environment key reached the log; the names alone say which services a machine talks to")
	}

	// The count is logged, because "was the overlay applied" is the one
	// question a debug session actually has about the environment.
	got := attrs(record)
	if n := got["env_overlay"].Int64(); n != 3 {
		t.Errorf("env_overlay = %d, want 3", n)
	}
	if n := got["stdout_bytes"].Int64(); n != int64(len(secretValue)) {
		t.Errorf("stdout_bytes = %d, want %d; the size is logged even though the bytes are not", n, len(secretValue))
	}
	// argv is logged in full, and deliberately: it is already readable through
	// /proc/<pid>/cmdline and Task Manager, so redacting it here would hide a
	// credential-on-a-command-line bug rather than remove it.
	if !strings.Contains(got["argv"].String(), "UPALL_TEST_TOKEN") {
		t.Error("argv was not logged in full")
	}
}

// TestNewDiscardsWhenGivenNoLogger pins the default. slog.Default writes to
// standard error, and a package below internal/cli that acquired a terminal by
// omission is exactly what the frontend boundary exists to prevent.
func TestNewDiscardsWhenGivenNoLogger(t *testing.T) {
	r, ok := New(nil).(*osRunner)
	if !ok {
		t.Fatalf("New returned %T, want *osRunner", New(nil))
	}

	if r.logger == nil {
		t.Fatal("New(nil) left the logger nil, which would panic on the first command")
	}
	if r.logger.Enabled(t.Context(), slog.LevelDebug) {
		t.Error("New(nil) produced a logger that is enabled; it must discard, not write to stderr")
	}
}

// testRunnerWithLogger is [testRunner] with a logger, for this file's tests.
func testRunnerWithLogger(logger *slog.Logger) *osRunner {
	r := testRunner(defaultKillGrace, defaultWaitDelay)
	r.logger = logger
	return r
}
