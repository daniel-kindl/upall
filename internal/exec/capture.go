package exec

// MaxCapture is how many bytes of each stream are kept.
//
// A subprocess upall does not control should not be able to exhaust memory by
// writing forever, so capture is bounded. The limit is far above anything a
// package manager produces — a large `apt list --upgradable` is tens of
// kilobytes — so reaching it means something has gone wrong, which is why
// [Output.Truncated] says so rather than letting the loss pass unmentioned.
const MaxCapture = 1 << 20 // 1 MiB

// retention is which end of an over-long stream a [capture] keeps.
type retention int

const (
	// keepHead keeps the first MaxCapture bytes, for stdout: a parser reads
	// from the beginning, so a truncated head is unparseable while a truncated
	// tail is merely incomplete.
	keepHead retention = iota

	// keepTail keeps the last MaxCapture bytes, for stderr: the error that
	// stopped a command is at the end, under whatever noise preceded it.
	keepTail
)

// capture accumulates one output stream, bounded to limit bytes.
//
// One capture belongs to one stream. os/exec copies stdout and stderr on
// separate goroutines, so two captures are written concurrently but never the
// same one, and it needs no lock of its own.
type capture struct {
	keep      retention
	limit     int
	buf       []byte
	truncated bool
}

// Write implements io.Writer. It never reports an error: a full capture drops
// what it cannot keep rather than failing the command, because losing the tail
// of some output is not a reason to abandon an upgrade that is working.
func (c *capture) Write(p []byte) (int, error) {
	n := len(p)

	switch c.keep {
	case keepHead:
		room := c.limit - len(c.buf)
		if len(p) > room {
			p, c.truncated = p[:max(room, 0)], true
		}
		c.buf = append(c.buf, p...)

	case keepTail:
		c.buf = append(c.buf, p...)
		// Compact at twice the limit rather than at it, so a command writing
		// megabytes one line at a time moves an amortised O(n) bytes instead of
		// O(n²).
		if len(c.buf) > 2*c.limit {
			c.compact()
		}
	}

	return n, nil
}

// compact drops all but the last limit bytes.
func (c *capture) compact() {
	if len(c.buf) <= c.limit {
		return
	}
	c.buf = c.buf[:copy(c.buf, c.buf[len(c.buf)-c.limit:])]
	c.truncated = true
}

// bytes returns what was kept, after a final compaction for a tail capture that
// has not reached its compaction threshold since the last write.
func (c *capture) bytes() []byte {
	if c.keep == keepTail {
		c.compact()
	}
	return c.buf
}
