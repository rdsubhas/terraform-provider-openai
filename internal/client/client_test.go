package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProjectRateLimitsReturnsAllPages(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organization/projects/proj_test/rate_limits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}

		switch r.URL.Query().Get("after") {
		case "":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"object":   "list",
				"data":     []map[string]interface{}{{"id": "rl-a", "object": "rate_limit", "model": "model-a"}},
				"last_id":  "rl-a",
				"has_more": true,
			})
		case "rl-a":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"object":   "list",
				"data":     []map[string]interface{}{{"id": "rl-b", "object": "rate_limit", "model": "model-b"}},
				"has_more": false,
			})
		default:
			t.Fatalf("unexpected after query: %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	c := NewClient("test-api-key", "", server.URL+"/v1")
	limits, err := c.GetProjectRateLimits("proj_test")
	if err != nil {
		t.Fatalf("GetProjectRateLimits returned error: %v", err)
	}
	if len(limits) != 2 {
		t.Fatalf("len(limits) = %d, want 2", len(limits))
	}
	if limits[0].Model != "model-a" || limits[1].Model != "model-b" {
		t.Fatalf("limits = %#v", limits)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
}
