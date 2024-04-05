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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type stats struct {
	RequestsCount   uint64 `json:"requestsCount"`
	ReceivedBytes   uint64 `json:"receivedBytes"`
	EarliestEpochMs int64  `json:"earliestEpochMs"`
	LatestEpochMs   int64  `json:"latestEpochMs"`
	DurationMs      uint64 `json:"durationMs"`
	CountPerSeconds uint64 `json:"countPerSeconds"`
	ClientsCount    int    `json:"clientsCount"`
}

type fileMetadata struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

type metadata struct {
	Scheme     string                    `json:"scheme"`
	Uri        string                    `json:"uri"`
	Path       string                    `json:"path"`
	Host       string                    `json:"host"`
	Version    string                    `json:"version"`
	Method     string                    `json:"method"`
	Parameters map[string][]string       `json:"parameters"`
	Headers    map[string][]string       `json:"headers"`
	Cookies    map[string]string         `json:"cookies"`
	Multipart  bool                      `json:"multipart"`
	Files      map[string][]fileMetadata `json:"files"`
	Size       int                       `json:"size"`
	Data       string                    `json:"data"`
	Form       map[string][]string       `json:"form"`
}

var remoteAddresses = make(map[string]struct{})
var empty struct{}
var remoteAddressesWriteMutex = sync.RWMutex{}
var requestsStats = stats{}

// Home returns the payload as response.
func Home(ctx *gin.Context) {
	now := time.Now().UnixMilli()
	atomic.CompareAndSwapInt64(&requestsStats.EarliestEpochMs, 0, now)
	atomic.AddUint64(&requestsStats.RequestsCount, 1)
	requestsStats.LatestEpochMs = now
	// Records the remote address as a unique connection.
	remoteAddressesWriteMutex.Lock()
	remoteAddresses[ctx.Request.RemoteAddr] = empty
	remoteAddressesWriteMutex.Unlock()

	message, err := ctx.GetRawData()
	atomic.AddUint64(&requestsStats.ReceivedBytes, uint64(len(message)))

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		ctx.JSON(http.StatusAccepted, gin.H{"binary": message})
	}
}

// RequestsStats returns the statistics of requests.
func RequestsStats(ctx *gin.Context) {
	requestsStats.ClientsCount = len(remoteAddresses)
	if requestsStats.LatestEpochMs > 0 {
		requestsStats.DurationMs = uint64(requestsStats.LatestEpochMs - requestsStats.EarliestEpochMs)
		requestsDuration := max(1, requestsStats.DurationMs/1000)
		requestsStats.CountPerSeconds = requestsStats.RequestsCount / requestsDuration
	}
	ctx.JSON(http.StatusOK, requestsStats)
}

// ResetStats resets the statistics of requests.
func ResetStats(ctx *gin.Context) {
	requestsStats = stats{}
	remoteAddresses = make(map[string]struct{})
	ctx.JSON(http.StatusOK, requestsStats)
}

// Describe the received requests.
func Describe(ctx *gin.Context) {
	requestMetadata := metadata{}
	request := ctx.Request
	requestMetadata.Uri = request.RequestURI
	requestMetadata.Path = request.URL.Path
	requestMetadata.Scheme = request.URL.Scheme
	requestMetadata.Host = request.Host
	requestMetadata.Method = request.Method
	requestMetadata.Version = request.Proto
	requestMetadata.Size = int(request.ContentLength)
	requestMetadata.Multipart = strings.Contains(request.Header.Get("Content-Type"), "multipart")

	requestMetadata.Parameters = map[string][]string{}
	for parameterName, value := range request.URL.Query() {
		requestMetadata.Parameters[parameterName] = value
	}

	requestMetadata.Headers = map[string][]string{}
	for headerName, value := range request.Header {
		if strings.ToLower(headerName) != "cookie" {
			requestMetadata.Headers[headerName] = value
		}
	}
	requestMetadata.Cookies = map[string]string{}
	for _, cookie := range request.Cookies() {
		requestMetadata.Cookies[cookie.Name] = cookie.Value
	}

	multipartForm, _ := ctx.MultipartForm()
	if multipartForm != nil {
		requestMetadata.Form = multipartForm.Value
		requestMetadata.Files = map[string][]fileMetadata{}
		for fileFieldName, files := range multipartForm.File {
			var metadata []fileMetadata
			for _, file := range files {
				metadata = append(metadata, fileMetadata{
					Name:        file.Filename,
					Size:        int(file.Size),
					ContentType: file.Header.Get("Content-Type"),
				})
			}
			requestMetadata.Files[fileFieldName] = metadata
		}
	} else {
		var data []byte
		buf := make([]byte, 1024)
		res := -1
		for {
			res, _ = request.Body.Read(buf)
			data = append(data, buf[0:res]...)
			if res == 0 {
				break
			}
		}

		requestMetadata.Data = string(data)
	}

	ctx.JSON(http.StatusOK, requestMetadata)
}
