package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestCachedProjectModelPermissions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetProjectSettingsCacheForTest()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/organization/projects/proj_test/model_permissions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-admin-key" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(ProjectModelPermissionsAPI{
			Mode: "deny_list", ModelIDs: []string{"o3"}, Object: "project.model_permissions",
		})
	}))
	defer server.Close()

	c := newTestOpenAIClient(server.URL)
	for i := 0; i < 2; i++ {
		value, status, err := cachedProjectModelPermissions(context.Background(), c, "proj_test")
		if err != nil || status != http.StatusOK {
			t.Fatalf("cachedProjectModelPermissions() status=%d error=%v", status, err)
		}
		if value.Mode != "deny_list" || len(value.ModelIDs) != 1 || value.ModelIDs[0] != "o3" {
			t.Fatalf("unexpected response: %#v", value)
		}
		if i == 0 {
			// Prove the second read can use the persistent cache, not only memory.
			resetProjectSettingsCacheForTest()
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one API call, got %d", calls.Load())
	}

	invalidateProjectModelPermissionsCache(c, "proj_test")
	_, _, err := cachedProjectModelPermissions(context.Background(), c, "proj_test")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected cache invalidation to trigger a second API call, got %d", calls.Load())
	}
}

func TestCachedProjectSpendAlert(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetProjectSettingsCacheForTest()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/organization/projects/proj_test/spend_alerts/alert_test" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ProjectSpendAlertAPI{
			ID: "alert_test", Object: "project.spend_alert", ThresholdAmount: 100000,
			Currency: "USD", Interval: "month",
			NotificationChannel: ProjectSpendAlertNotificationAPI{Type: "email", Recipients: []string{"finance@example.com"}},
		})
	}))
	defer server.Close()

	c := newTestOpenAIClient(server.URL)
	for i := 0; i < 2; i++ {
		value, status, err := cachedProjectSpendAlert(context.Background(), c, "proj_test", "alert_test")
		if err != nil || status != http.StatusOK {
			t.Fatalf("cachedProjectSpendAlert() status=%d error=%v", status, err)
		}
		if value.ID != "alert_test" || value.ThresholdAmount != 100000 {
			t.Fatalf("unexpected response: %#v", value)
		}
		if i == 0 {
			resetProjectSettingsCacheForTest()
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one API call, got %d", calls.Load())
	}
}

func TestProjectSettingsResourcesRegistered(t *testing.T) {
	wanted := map[string]bool{
		"openai_project_model_permissions": false,
		"openai_project_spend_alert":       false,
	}
	p := &FrameworkProvider{}
	for _, constructor := range p.Resources(context.Background()) {
		var response resource.MetadataResponse
		constructor().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "openai"}, &response)
		if _, ok := wanted[response.TypeName]; ok {
			wanted[response.TypeName] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("resource %s is not registered", name)
		}
	}
}
