// progress.go provides a minimal stderr progress renderer for long-running list commands.
package cmd

import (
	"fmt"
	"io"
	"sync"
)

type progressReporter struct {
	enabled  bool
	writer   io.Writer
	resource string

	mu       sync.Mutex
	lineOpen bool
}

func newProgressReporter(writer io.Writer, resource string, enabled bool) *progressReporter {
	return &progressReporter{
		enabled:  enabled,
		writer:   writer,
		resource: resource,
	}
}

func (p *progressReporter) Update(completed int, total int) {
	if !p.enabled || total <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	_, _ = fmt.Fprintf(
		p.writer,
		"\rFetching %s %d/%d",
		p.resource,
		completed,
		total,
	)

	if completed >= total {
		_, _ = fmt.Fprintln(p.writer)
		p.lineOpen = false
		return
	}
	p.lineOpen = true
}

func (p *progressReporter) Done() {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.lineOpen {
		return
	}
	_, _ = fmt.Fprintln(p.writer)
	p.lineOpen = false
}
