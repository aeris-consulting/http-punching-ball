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
	"github.com/gin-gonic/gin"
	"net/http"
	"sync/atomic"
	"time"
)

type stats struct {
	Count           uint64 `json:"count"`
	PayloadBytes    uint64 `json:"payloadBytes"`
	EarliestEpochMs int64  `json:"earliestEpochMs"`
	LatestEpochMs   int64  `json:"latestEpochMs"`
}

var requestsStats = stats{}

// Home returns the payload as response.
func Home(ctx *gin.Context) {
	now := time.Now().UnixMilli()
	atomic.CompareAndSwapInt64(&requestsStats.EarliestEpochMs, 0, now)
	atomic.AddUint64(&requestsStats.Count, 1)
	requestsStats.LatestEpochMs = now

	message, err := ctx.GetRawData()
	atomic.AddUint64(&requestsStats.PayloadBytes, uint64(len(message)))

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		ctx.JSON(http.StatusAccepted, gin.H{"binary": message})
	}
}

// RequestsStats returns the statistics of requests.
func RequestsStats(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, requestsStats)
}

// ResetStats resets the statistics of requests.
func ResetStats(ctx *gin.Context) {
	requestsStats = stats{}
	ctx.JSON(http.StatusOK, requestsStats)
}
