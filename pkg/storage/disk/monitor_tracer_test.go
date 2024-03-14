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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonitorTracer_recordEvent(t *testing.T) {
	tracer := newMonitorTracer(time.Second, 3)
	tracer.recordEvent(traceEvent{
		time:  time.Now(),
		stats: Stats{},
		err:   nil,
	})
	tracer.recordEvent(traceEvent{
		time:  time.Now(),
		stats: Stats{},
		err:   nil,
	})
	tracer.recordEvent(traceEvent{
		time:  time.Now(),
		stats: Stats{},
		err:   nil,
	})
	tracer.recordEvent(traceEvent{
		time:  time.Now().Add(15 * time.Minute),
		stats: Stats{},
		err:   nil,
	})
	fmt.Println(tracer.String())
	fmt.Println(tracer.String())
}

func TestMonitorTracer_find(t *testing.T) {

}

func TestMonitorTracer_removeExpired(t *testing.T) {
	events := []traceEvent{
		{
			time:  time.Unix(10, 0),
			stats: Stats{},
			err:   nil,
		},
		{
			time:  time.Unix(40, 0),
			stats: Stats{},
			err:   nil,
		},
		{
			time:  time.Unix(70, 0),
			stats: Stats{},
			err:   nil,
		},
		{
			time:  time.Unix(100, 0),
			stats: Stats{},
			err:   nil,
		},
	}

	testCases := []struct {
		name      string
		expiry    time.Time
		wantSize  int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "trace history with equal timestamp",
			expiry:    time.Unix(70, 0),
			wantSize:  2,
			wantStart: 2,
			wantEnd:   1,
		},
		{
			name:      "remove entire trace history",
			expiry:    time.Unix(101, 0),
			wantSize:  0,
			wantStart: 1,
			wantEnd:   1,
		},
		{
			name:      "no removals",
			expiry:    time.Unix(0, 0),
			wantSize:  3,
			wantStart: 1,
			wantEnd:   1,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			tracer := newMonitorTracer(time.Since(time.Time{}), 3)
			for _, event := range events {
				tracer.recordEvent(event)
			}
			tracer.removeExpired(test.expiry)

			require.Equal(t, tracer.mu.size, test.wantSize)
			require.Equal(t, tracer.mu.start, test.wantStart)
			require.Equal(t, tracer.mu.end, test.wantEnd)
		})
	}
}
