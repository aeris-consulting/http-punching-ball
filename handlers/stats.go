//    Copyright 2021 AERIS-Consulting e.U.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package handlers

import (
	"context"
	"net"
	"sync/atomic"
)

// stats is the JSON representation of the statistics returned by /_stats.
type stats struct {
	RequestsCount   uint64 `json:"requestsCount"`
	ReceivedBytes   uint64 `json:"receivedBytes"`
	EarliestEpochMs int64  `json:"earliestEpochMs"`
	LatestEpochMs   int64  `json:"latestEpochMs"`
	DurationMs      uint64 `json:"durationMs"`
	CountPerSeconds uint64 `json:"countPerSeconds"`
	ClientsCount    int    `json:"clientsCount"`
}

// counters holds the lock-free statistics of one measurement period. A reset replaces the whole instance
// atomically, so that a request is always accounted entirely in one period.
type counters struct {
	requestsCount   atomic.Uint64
	receivedBytes   atomic.Uint64
	earliestEpochMs atomic.Int64
	latestEpochMs   atomic.Int64
	clientsCount    atomic.Int64
}

// connection tracks the last measurement period in which a connection was counted as a client.
type connection struct {
	countedIn atomic.Pointer[counters]
}

type connectionKey struct{}

var current atomic.Pointer[counters]

func init() {
	current.Store(&counters{})
}

// ConnContext attaches a tracker to each new connection, so that the distinct connections sending requests to /
// are counted without any lock nor per-request map. To use as http.Server.ConnContext.
func ConnContext(ctx context.Context, _ net.Conn) context.Context {
	return context.WithValue(ctx, connectionKey{}, &connection{})
}

func (c *counters) record(ctx context.Context, nowMs int64, bodySize int) {
	c.requestsCount.Add(1)
	c.receivedBytes.Add(uint64(bodySize))
	c.earliestEpochMs.CompareAndSwap(0, nowMs)
	for latest := c.latestEpochMs.Load(); latest < nowMs; latest = c.latestEpochMs.Load() {
		if c.latestEpochMs.CompareAndSwap(latest, nowMs) {
			break
		}
	}
	if conn, ok := ctx.Value(connectionKey{}).(*connection); ok {
		if previous := conn.countedIn.Load(); previous != c && conn.countedIn.CompareAndSwap(previous, c) {
			c.clientsCount.Add(1)
		}
	}
}

func (c *counters) snapshot() stats {
	result := stats{
		RequestsCount:   c.requestsCount.Load(),
		ReceivedBytes:   c.receivedBytes.Load(),
		EarliestEpochMs: c.earliestEpochMs.Load(),
		LatestEpochMs:   c.latestEpochMs.Load(),
		ClientsCount:    int(c.clientsCount.Load()),
	}
	if result.LatestEpochMs > 0 && result.EarliestEpochMs > 0 {
		result.DurationMs = uint64(result.LatestEpochMs - result.EarliestEpochMs)
		result.CountPerSeconds = result.RequestsCount / max(1, result.DurationMs/1000)
	}
	return result
}
