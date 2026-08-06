package exec

import (
	"strings"
	"testing"
)

func TestCaptureBounds(t *testing.T) {
	tests := []struct {
		name          string
		keep          retention
		limit         int
		writes        []string
		want          string
		wantTruncated bool
	}{
		{
			name:   "under the limit nothing is dropped",
			keep:   keepHead,
			limit:  8,
			writes: []string{"abc", "de"},
			want:   "abcde",
		},
		{
			name:   "exactly at the limit nothing is dropped",
			keep:   keepHead,
			limit:  5,
			writes: []string{"abcde"},
			want:   "abcde",
		},
		{
			// stdout keeps its beginning: a parser reads from the front, so a
			// missing head is unparseable while a missing tail is merely short.
			name:          "a head capture keeps the beginning",
			keep:          keepHead,
			limit:         5,
			writes:        []string{"abcdefghij"},
			want:          "abcde",
			wantTruncated: true,
		},
		{
			name:          "a head capture drops everything after it is full",
			keep:          keepHead,
			limit:         4,
			writes:        []string{"ab", "cd", "ef", "gh"},
			want:          "abcd",
			wantTruncated: true,
		},
		{
			// stderr keeps its end: the error that stopped a command is the
			// last thing it said, under whatever noise came before.
			name:          "a tail capture keeps the end",
			keep:          keepTail,
			limit:         5,
			writes:        []string{"abcdefghij"},
			want:          "fghij",
			wantTruncated: true,
		},
		{
			// Compaction happens at twice the limit rather than at it, so this
			// crosses the threshold several times and must still land on the
			// last few bytes.
			name:          "a tail capture compacts repeatedly and still ends correctly",
			keep:          keepTail,
			limit:         4,
			writes:        []string{"ab", "cd", "ef", "gh", "ij", "kl", "mn", "op"},
			want:          "mnop",
			wantTruncated: true,
		},
		{
			name:   "a tail capture under the limit is untouched",
			keep:   keepTail,
			limit:  8,
			writes: []string{"ab", "cd"},
			want:   "abcd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &capture{keep: tt.keep, limit: tt.limit}
			for _, w := range tt.writes {
				n, err := c.Write([]byte(w))
				if err != nil {
					t.Fatalf("Write(%q) returned %v, want no error", w, err)
				}
				// The io.Writer contract is that a short write is an error. A
				// capture that reported one would abort the copy and, through
				// it, fail a command that is running perfectly well.
				if n != len(w) {
					t.Fatalf("Write(%q) = %d, want %d; dropping bytes must not look like a short write", w, n, len(w))
				}
			}

			if got := string(c.bytes()); got != tt.want {
				t.Errorf("captured %q, want %q", got, tt.want)
			}
			if c.truncated != tt.wantTruncated {
				t.Errorf("truncated = %t, want %t", c.truncated, tt.wantTruncated)
			}
		})
	}
}

// TestCaptureHoldsNoMoreThanTwiceTheLimit checks the thing the bound exists for.
// A command writing far more than the limit must not be able to grow the buffer
// without end, which is the failure mode that makes an unbounded capture a
// denial of service by a subprocess upall does not control.
func TestCaptureHoldsNoMoreThanTwiceTheLimit(t *testing.T) {
	const limit = 64

	for _, keep := range []retention{keepHead, keepTail} {
		c := &capture{keep: keep, limit: limit}
		for range 1000 {
			if _, err := c.Write([]byte(strings.Repeat("x", 32))); err != nil {
				t.Fatalf("Write returned %v, want no error", err)
			}
			if len(c.buf) > 2*limit {
				t.Fatalf("the buffer grew to %d bytes with a limit of %d", len(c.buf), limit)
			}
		}
		if got := len(c.bytes()); got != limit {
			t.Errorf("after 32000 bytes the capture kept %d, want %d", got, limit)
		}
	}
}
