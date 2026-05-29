package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/mkdev-me/terraform-provider-openai/internal/client"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"openai": providerserver.NewProtocol6WithError(NewFrameworkProvider("test")()),
}

func readRateLimitForTest(t *testing.T, r *RateLimitResource, schema rschema.Schema, projectID, model string) resource.ReadResponse {
	t.Helper()

	ctx := context.Background()
	state := tfsdk.State{Schema: schema}
	diags := state.Set(ctx, &RateLimitResourceModel{
		ID:          types.StringValue("rl-" + model),
		RateLimitID: types.StringValue("rl-" + model),
		ProjectID:   types.StringValue(projectID),
		Model:       types.StringValue(model),
	})
	if diags.HasError() {
		t.Fatalf("could not build state: %v", diags)
	}

	resp := resource.ReadResponse{State: tfsdk.State{Schema: schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read produced diagnostics: %v", resp.Diagnostics)
	}
	return resp
}

func init() {
	// Provider initialization is now handled in testAccProtoV6ProviderFactories
}

// testAccPreCheck ensures the necessary environment variables are set for acceptance tests
func testAccPreCheck(t *testing.T) {
	// Verify that required environment variables are set for acceptance tests
	if v := os.Getenv("OPENAI_API_KEY"); v == "" {
		t.Fatal("OPENAI_API_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("OPENAI_ORGANIZATION_ID"); v == "" {
		t.Fatal("OPENAI_ORGANIZATION_ID must be set for acceptance tests")
	}
}

// testClient returns a client for use in unit tests
func testClient() *client.OpenAIClient {
	return client.NewClient(
		os.Getenv("OPENAI_API_KEY"),
		os.Getenv("OPENAI_ORGANIZATION_ID"),
		"https://api.openai.com/v1",
	)
}

// TestRateLimitResourceReadPreservesNullOptionalIntegers verifies null API rate limit values stay null in Terraform state.
func TestRateLimitResourceReadPreservesNullOptionalIntegers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Path == "/v1" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organization/projects/proj_test/rate_limits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"id":                               "rl-gpt-test",
					"object":                           "rate_limit",
					"model":                            "gpt-test",
					"max_requests_per_1_minute":        nil,
					"max_tokens_per_1_minute":          nil,
					"max_images_per_1_minute":          nil,
					"batch_1_day_max_input_tokens":     nil,
					"max_audio_megabytes_per_1_minute": nil,
					"max_requests_per_1_day":           nil,
				},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	ctx := context.Background()
	r := &RateLimitResource{
		client: client.NewClient("test-api-key", "", server.URL+"/v1"),
	}
	schema := currentSchema(t, r)

	state := tfsdk.State{Schema: schema}
	diags := state.Set(ctx, &RateLimitResourceModel{
		ID:                          types.StringValue("rl-gpt-test"),
		RateLimitID:                 types.StringValue("rl-gpt-test"),
		ProjectID:                   types.StringValue("proj_test"),
		Model:                       types.StringValue("gpt-test"),
		MaxRequestsPerMinute:        types.Int64Null(),
		MaxTokensPerMinute:          types.Int64Null(),
		MaxImagesPerMinute:          types.Int64Null(),
		Batch1DayMaxInputTokens:     types.Int64Null(),
		MaxAudioMegabytesPer1Minute: types.Int64Null(),
		MaxRequestsPer1Day:          types.Int64Null(),
	})
	if diags.HasError() {
		t.Fatalf("could not build state: %v", diags)
	}

	resp := resource.ReadResponse{State: tfsdk.State{Schema: schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read produced diagnostics: %v", resp.Diagnostics)
	}

	var got RateLimitResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("could not read state: %v", diags)
	}

	if !got.MaxRequestsPerMinute.IsNull() {
		t.Fatalf("MaxRequestsPerMinute = %v, want null", got.MaxRequestsPerMinute)
	}
	if !got.MaxTokensPerMinute.IsNull() {
		t.Fatalf("MaxTokensPerMinute = %v, want null", got.MaxTokensPerMinute)
	}
	if !got.MaxImagesPerMinute.IsNull() {
		t.Fatalf("MaxImagesPerMinute = %v, want null", got.MaxImagesPerMinute)
	}
	if !got.Batch1DayMaxInputTokens.IsNull() {
		t.Fatalf("Batch1DayMaxInputTokens = %v, want null", got.Batch1DayMaxInputTokens)
	}
	if !got.MaxAudioMegabytesPer1Minute.IsNull() {
		t.Fatalf("MaxAudioMegabytesPer1Minute = %v, want null", got.MaxAudioMegabytesPer1Minute)
	}
	if !got.MaxRequestsPer1Day.IsNull() {
		t.Fatalf("MaxRequestsPer1Day = %v, want null", got.MaxRequestsPer1Day)
	}
}

func TestRateLimitResourceReadUsesProjectCache(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organization/projects/proj_test/rate_limits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		atomic.AddInt32(&requests, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "rl-model-a", "object": "rate_limit", "model": "model-a"},
				{"id": "rl-model-b", "object": "rate_limit", "model": "model-b"},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	r := &RateLimitResource{
		client:         client.NewClient("test-api-key", "", server.URL+"/v1"),
		rateLimitCache: newRateLimitProjectCache(30 * time.Second),
	}
	schema := currentSchema(t, r)

	readRateLimitForTest(t, r, schema, "proj_test", "model-a")
	readRateLimitForTest(t, r, schema, "proj_test", "model-b")

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestRateLimitResourceReadCollapsesConcurrentProjectCacheMiss(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organization/projects/proj_test/rate_limits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		atomic.AddInt32(&requests, 1)
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "rl-model-a", "object": "rate_limit", "model": "model-a"},
				{"id": "rl-model-b", "object": "rate_limit", "model": "model-b"},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	r := &RateLimitResource{
		client:         client.NewClient("test-api-key", "", server.URL+"/v1"),
		rateLimitCache: newRateLimitProjectCache(30 * time.Second),
	}
	schema := currentSchema(t, r)

	var wg sync.WaitGroup
	for _, model := range []string{"model-a", "model-b"} {
		wg.Add(1)
		go func(model string) {
			defer wg.Done()
			readRateLimitForTest(t, r, schema, "proj_test", model)
		}(model)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestRateLimitResourceProjectCacheTTLExpires(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organization/projects/proj_test/rate_limits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		atomic.AddInt32(&requests, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "rl-model-a", "object": "rate_limit", "model": "model-a"},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	r := &RateLimitResource{
		client:         client.NewClient("test-api-key", "", server.URL+"/v1"),
		rateLimitCache: newRateLimitProjectCache(10 * time.Millisecond),
	}
	schema := currentSchema(t, r)

	readRateLimitForTest(t, r, schema, "proj_test", "model-a")
	time.Sleep(20 * time.Millisecond)
	readRateLimitForTest(t, r, schema, "proj_test", "model-a")

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRateLimitResourceInvalidateProjectCache(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organization/projects/proj_test/rate_limits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		atomic.AddInt32(&requests, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "rl-model-a", "object": "rate_limit", "model": "model-a"},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	r := &RateLimitResource{
		client:         client.NewClient("test-api-key", "", server.URL+"/v1"),
		rateLimitCache: newRateLimitProjectCache(30 * time.Second),
	}
	schema := currentSchema(t, r)

	readRateLimitForTest(t, r, schema, "proj_test", "model-a")
	r.invalidateRateLimitCache("proj_test")
	readRateLimitForTest(t, r, schema, "proj_test", "model-a")

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRateLimitResourceUpdateInvalidatesProjectCache(t *testing.T) {
	var listRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organization/projects/proj_test/rate_limits":
			atomic.AddInt32(&listRequests, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"id": "rl-model-a", "object": "rate_limit", "model": "model-a"},
				},
				"has_more": false,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/organization/projects/proj_test/rate_limits/rl-model-a":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "rl-model-a",
				"object":  "rate_limit",
				"model":   "model-a",
				"updated": true,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	r := &RateLimitResource{
		client:         client.NewClient("test-api-key", "", server.URL+"/v1"),
		rateLimitCache: newRateLimitProjectCache(30 * time.Second),
	}
	schema := currentSchema(t, r)

	readRateLimitForTest(t, r, schema, "proj_test", "model-a")

	ctx := context.Background()
	plan := tfsdk.Plan{Schema: schema}
	diags := plan.Set(ctx, &RateLimitResourceModel{
		ID:                   types.StringValue("rl-model-a"),
		RateLimitID:          types.StringValue("rl-model-a"),
		ProjectID:            types.StringValue("proj_test"),
		Model:                types.StringValue("model-a"),
		MaxRequestsPerMinute: types.Int64Value(123),
	})
	if diags.HasError() {
		t.Fatalf("could not build plan: %v", diags)
	}

	updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("Update produced diagnostics: %v", updateResp.Diagnostics)
	}

	readRateLimitForTest(t, r, schema, "proj_test", "model-a")

	if got := atomic.LoadInt32(&listRequests); got != 3 {
		t.Fatalf("listRequests = %d, want 3", got)
	}
}
