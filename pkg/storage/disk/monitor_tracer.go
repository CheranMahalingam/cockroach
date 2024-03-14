// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package disk

import (
	"bytes"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
)

type traceEvent struct {
	time  time.Time
	stats Stats
	err   error
}

func (t *traceEvent) String() string {
	if t.err != nil {
		return fmt.Sprintf("%s\t\t%s", t.time.Format(time.RFC3339Nano), t.err.Error())
	}
	return fmt.Sprintf("%s\t%s\tnil", t.time.Format(time.RFC3339Nano), t.stats.String())
}

// monitorTracer manages a circular queue containing a history of disk stats.
// The tracer is designed such that higher-level components can apply aggregation
// functions to compute statistics over rolling windows and output detailed disk
// traces when failures are detected.
type monitorTracer struct {
	period   time.Duration
	capacity int

	mu struct {
		syncutil.Mutex
		trace []traceEvent
		start int
		end   int
		size  int
	}
}

func newMonitorTracer(period time.Duration, capacity int) *monitorTracer {
	return &monitorTracer{
		period:   period,
		capacity: capacity,
		mu: struct {
			syncutil.Mutex
			trace []traceEvent
			start int
			end   int
			size  int
		}{
			trace: make([]traceEvent, capacity),
			start: 0,
			end:   0,
			size:  0,
		},
	}
}

func (m *monitorTracer) recordEvent(event traceEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeExpired(timeutil.Now().Add(-m.period))
	m.mu.trace[m.mu.end] = event
	m.mu.end = (m.mu.end + 1) % m.capacity
	if m.mu.size == m.capacity {
		m.mu.start = (m.mu.start + 1) % m.capacity
	} else {
		m.mu.size++
	}
}

// find retrieves stats from the traceEvent that occurred before and closest to
// specified time, t. If no events occurred before t, an error is thrown.
func (m *monitorTracer) find(t time.Time) (traceEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mu.size == 0 {
		return traceEvent{}, errors.Errorf("trace is empty")
	}

	//var size int
	//if m.mu.end < m.mu.start {
	//	size = (m.capacity - m.mu.start) + m.mu.end
	//} else {
	//	size = m.mu.end - m.mu.start
	//}
	//// Apply binary search to find the first traceEvent that occurred at or after time t.
	//offset, found := sort.Find(size, func(i int) int {
	//	idx := (m.mu.start + i) % m.capacity
	//	return t.Compare(m.mu.trace[idx].time)
	//})
	//traceIdx := (m.mu.start + offset) % m.capacity
	//if found {
	//	return m.mu.trace[traceIdx].stats, nil
	//}
	//if offset == 0 {
	//	return Stats{}, errors.Errorf("no event found in trace before time %s", t.String())
	//}
	//return m.mu.trace[traceIdx-1].stats, nil

	findIdx := -1
	for i := 0; i < m.mu.size; i++ {
		traceIdx := (m.mu.start + i) % m.capacity
		if m.mu.trace[traceIdx].time.After(t) {
			break
		}
		findIdx = traceIdx
	}
	if findIdx == -1 {
		return traceEvent{}, errors.Errorf("no event found in trace before time %s", t.String())
	}
	return m.mu.trace[findIdx], nil
}

// latest retrieves stats from the last traceEvent that was queued. If the trace
// is empty or if there was an error collecting the previous traceEvent, we throw
// an error.
func (m *monitorTracer) latest() (traceEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mu.size == 0 {
		return traceEvent{}, errors.Errorf("trace is empty")
	}
	return m.mu.trace[m.mu.end], nil
}

func (m *monitorTracer) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mu.size == 0 {
		return ""
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 2, 1, 2, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "Time\t"+
		"Device Name\tReads Completed\tReads Merged\tSectors Read\tRead Duration\t"+
		"Writes Completed\tWrites Merged\tSectors Written\tWrite Duration\t"+
		"IO in Progress\tIO Duration\tWeighted IO Duration\t"+
		"Discards Completed\tDiscards Merged\tSectors Discarded\tDiscard Duration\t"+
		"Flushes Completed\tFlush Duration\tError")
	prevStats := m.mu.trace[m.mu.start].stats
	for i := 1; i < m.mu.size; i++ {
		event := m.mu.trace[(m.mu.start+i)%m.capacity]
		delta := event.stats.delta(&prevStats)
		if event.err == nil {
			prevStats = event.stats
		}
		deltaEvent := traceEvent{
			time:  event.time,
			stats: delta,
			err:   event.err,
		}
		fmt.Fprintln(w, deltaEvent.String())
	}
	_ = w.Flush()

	return buf.String()
}

// Removes events from trace that occurred before a specified time, t. Note that it
// is the responsibility of the caller to acquire the tracer mutex.
func (m *monitorTracer) removeExpired(t time.Time) {
	if m.mu.size == 0 {
		return
	}
	traceLen := m.mu.size
	for i := 0; i < traceLen; i++ {
		event := m.mu.trace[m.mu.start]
		if !event.time.Before(t) {
			break
		}
		m.mu.start = (m.mu.start + 1) % m.capacity
		m.mu.size--
	}
}
