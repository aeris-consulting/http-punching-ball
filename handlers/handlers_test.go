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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Any("/", Home)
	router.GET("/_stats", RequestsStats)
	router.DELETE("/_stats", ResetStats)
	router.NoRoute(Describe)

	server := httptest.NewUnstartedServer(router)
	server.Config.ConnContext = ConnContext
	server.Start()
	t.Cleanup(server.Close)
	current.Store(&counters{})
	return server
}

// send executes a request and fails the test without stopping it, so that it can be called from goroutines.
func send(t *testing.T, client *http.Client, method, url, body string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Error(err)
		return 0, nil
	}
	response, err := client.Do(request)
	if err != nil {
		t.Error(err)
		return 0, nil
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Error(err)
	}
	return response.StatusCode, payload
}

func fetchStats(t *testing.T, client *http.Client, server *httptest.Server, method string) stats {
	t.Helper()
	status, payload := send(t, client, method, server.URL+"/_stats", "")
	var result stats
	if status != http.StatusOK {
		t.Errorf("unexpected status %d for %s /_stats", status, method)
	} else if err := json.Unmarshal(payload, &result); err != nil {
		t.Error(err)
	}
	return result
}

// newClient returns a client with its own pool of connections.
func newClient() *http.Client {
	return &http.Client{Transport: &http.Transport{}}
}

func TestHomeEchoesThePayload(t *testing.T) {
	server := newTestServer(t)

	status, payload := send(t, server.Client(), http.MethodPost, server.URL+"/", "hello")

	if status != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, status)
	}
	var result struct {
		Binary []byte `json:"binary"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	if string(result.Binary) != "hello" {
		t.Fatalf("expected the echo of the payload, got %q", result.Binary)
	}
}

func TestStatsCountTheRequestsAndBytes(t *testing.T) {
	server := newTestServer(t)
	client := server.Client()
	const requests = 50

	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() { send(t, client, http.MethodPut, server.URL+"/", "1234") })
	}
	wg.Wait()
	result := fetchStats(t, client, server, http.MethodGet)

	if result.RequestsCount != requests {
		t.Errorf("expected %d requests, got %d", requests, result.RequestsCount)
	}
	if result.ReceivedBytes != 4*requests {
		t.Errorf("expected %d bytes, got %d", 4*requests, result.ReceivedBytes)
	}
	if result.EarliestEpochMs == 0 || result.LatestEpochMs < result.EarliestEpochMs {
		t.Errorf("unexpected timestamps %d - %d", result.EarliestEpochMs, result.LatestEpochMs)
	}
	if result.DurationMs != uint64(result.LatestEpochMs-result.EarliestEpochMs) {
		t.Errorf("unexpected duration %d", result.DurationMs)
	}
	if result.CountPerSeconds == 0 {
		t.Errorf("expected a non-zero count per seconds")
	}
}

func TestStatsCountTheDistinctConnections(t *testing.T) {
	server := newTestServer(t)
	clients := []*http.Client{newClient(), newClient(), newClient()}

	for _, client := range clients {
		for range 3 {
			send(t, client, http.MethodGet, server.URL+"/", "")
		}
	}
	// Requests to the statistics do not count as clients.
	result := fetchStats(t, newClient(), server, http.MethodGet)

	if result.RequestsCount != 9 {
		t.Errorf("expected 9 requests, got %d", result.RequestsCount)
	}
	if result.ClientsCount != len(clients) {
		t.Errorf("expected %d clients, got %d", len(clients), result.ClientsCount)
	}
}

func TestResetClearsTheStatsAndRecountsTheOpenConnections(t *testing.T) {
	server := newTestServer(t)
	client := newClient()
	send(t, client, http.MethodPost, server.URL+"/", "abc")

	reset := fetchStats(t, client, server, http.MethodDelete)
	afterReset := fetchStats(t, client, server, http.MethodGet)

	if reset != (stats{}) || afterReset != (stats{}) {
		t.Fatalf("expected empty stats after the reset, got %+v and %+v", reset, afterReset)
	}
	send(t, client, http.MethodPost, server.URL+"/", "de")
	result := fetchStats(t, client, server, http.MethodGet)
	if result.RequestsCount != 1 || result.ReceivedBytes != 2 || result.ClientsCount != 1 {
		t.Fatalf("expected the reused connection to be counted again, got %+v", result)
	}
}

func TestResetIsSafeUnderConcurrentRequests(t *testing.T) {
	server := newTestServer(t)
	client := server.Client()

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 10 {
				send(t, client, http.MethodPost, server.URL+"/", "x")
			}
		})
	}
	for range 5 {
		wg.Go(func() { fetchStats(t, client, server, http.MethodDelete) })
		wg.Go(func() { fetchStats(t, client, server, http.MethodGet) })
	}
	wg.Wait()

	result := fetchStats(t, client, server, http.MethodGet)
	if result.ReceivedBytes != result.RequestsCount || result.RequestsCount > 200 {
		t.Fatalf("inconsistent stats %+v", result)
	}
}

func TestDescribeReturnsTheRequestMetadata(t *testing.T) {
	server := newTestServer(t)

	status, payload := send(t, server.Client(), http.MethodPost, server.URL+"/any/path?key=value", "some data")

	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	var result metadata
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != "/any/path" || result.Method != http.MethodPost || result.Data != "some data" ||
		result.Parameters["key"][0] != "value" {
		t.Fatalf("unexpected metadata %+v", result)
	}
}
