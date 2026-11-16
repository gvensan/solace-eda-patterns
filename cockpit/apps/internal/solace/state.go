package solace

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Holding reports what an app holds (a copy, a read model, a database) to
// the dashboard, which shows it as a table under the diagram. A node shows it
// when its yaml has a holds: entry.
//
// Each report is one stdout line, "@state {json}", which the dashboard keeps
// as the latest and leaves out of the log. Reports are at most twice a
// second; a change inside that gap is still reported at the end of it, so
// the last state is never lost.
type Holding struct {
	title   string
	columns []string

	mu      sync.Mutex
	rows    [][]string
	note    string
	last    time.Time
	pending bool
}

const holdingGap = 500 * time.Millisecond

// NewHolding starts a report with the given title and columns, and reports
// it empty at once, so the page shows "holds nothing" rather than nothing.
func NewHolding(title string, columns ...string) *Holding {
	h := &Holding{title: title, columns: columns}
	h.Set(nil, "")
	return h
}

// Set replaces every row, and the note shown under them ("" for none). Rows
// are shown in the order given; the caller must not change them afterwards.
func (h *Holding) Set(rows [][]string, note string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rows, h.note = rows, note
	wait := holdingGap - time.Since(h.last)
	if wait <= 0 {
		h.printLocked()
		return
	}
	if !h.pending {
		h.pending = true
		time.AfterFunc(wait, func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.pending = false
			h.printLocked()
		})
	}
}

func (h *Holding) printLocked() {
	rows := h.rows
	if rows == nil {
		rows = [][]string{}
	}
	b, err := json.Marshal(map[string]any{"title": h.title, "columns": h.columns, "rows": rows, "note": h.note})
	if err != nil {
		return
	}
	fmt.Printf("@state %s\n", b)
	h.last = time.Now()
}

// WriteFileAtomic writes the whole file beside the target and renames it into
// place, so a kill mid-write leaves the previous contents, never half a file
// or an empty one.
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
