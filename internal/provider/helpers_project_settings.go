package provider

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const projectSettingsCacheTTL = 30 * time.Second

type ProjectModelPermissionsAPI struct {
	Mode     string   `json:"mode"`
	ModelIDs []string `json:"model_ids"`
	Object   string   `json:"object"`
}

type ProjectSpendAlertNotificationAPI struct {
	Type          string   `json:"type"`
	Recipients    []string `json:"recipients"`
	SubjectPrefix *string  `json:"subject_prefix,omitempty"`
}

type ProjectSpendAlertAPI struct {
	ID                  string                           `json:"id"`
	Object              string                           `json:"object"`
	ThresholdAmount     int64                            `json:"threshold_amount"`
	Currency            string                           `json:"currency"`
	Interval            string                           `json:"interval"`
	NotificationChannel ProjectSpendAlertNotificationAPI `json:"notification_channel"`
}

type projectSettingsCacheEntry[T any] struct {
	Value     T         `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

var projectSettingsCache = struct {
	sync.Mutex
	modelPermissions map[string]projectSettingsCacheEntry[ProjectModelPermissionsAPI]
	spendAlerts      map[string]projectSettingsCacheEntry[ProjectSpendAlertAPI]
}{
	modelPermissions: map[string]projectSettingsCacheEntry[ProjectModelPermissionsAPI]{},
	spendAlerts:      map[string]projectSettingsCacheEntry[ProjectSpendAlertAPI]{},
}

func spendAlertCacheKey(projectID, alertID string) string {
	return projectID + ":" + alertID
}

func projectSettingsIdentityKey(c *OpenAIClient, values ...string) string {
	parts := []string{adminBaseURL(c), c.OrganizationID, adminAPIKey(c)}
	parts = append(parts, values...)
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", hash)
}

func invalidateProjectModelPermissionsCache(c *OpenAIClient, projectID string) {
	key := projectSettingsIdentityKey(c, projectID, "model_permissions")
	projectSettingsCache.Lock()
	delete(projectSettingsCache.modelPermissions, key)
	projectSettingsCache.Unlock()
	_ = os.Remove(projectSettingsCachePath("model-permissions", key))
}

func invalidateProjectSpendAlertCache(c *OpenAIClient, projectID, alertID string) {
	key := projectSettingsIdentityKey(c, spendAlertCacheKey(projectID, alertID), "spend_alert")
	projectSettingsCache.Lock()
	delete(projectSettingsCache.spendAlerts, key)
	projectSettingsCache.Unlock()
	_ = os.Remove(projectSettingsCachePath("spend-alerts", key))
}

func resetProjectSettingsCacheForTest() {
	projectSettingsCache.Lock()
	projectSettingsCache.modelPermissions = map[string]projectSettingsCacheEntry[ProjectModelPermissionsAPI]{}
	projectSettingsCache.spendAlerts = map[string]projectSettingsCacheEntry[ProjectSpendAlertAPI]{}
	projectSettingsCache.Unlock()
}

func projectSettingsRequest(ctx context.Context, c *OpenAIClient, method, path string, requestBody any, responseBody any) (int, error) {
	if c == nil || c.OpenAIClient == nil || adminAPIKey(c) == "" {
		return 0, fmt.Errorf("admin API key is required")
	}

	var body []byte
	var err error
	if requestBody != nil {
		body, err = json.Marshal(requestBody)
		if err != nil {
			return 0, fmt.Errorf("error marshaling request: %w", err)
		}
	}

	resp, err := doRequestWithRetry(ctx, projectClientHTTP(c), c, method, adminBaseURL(c)+path, body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("error reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("OpenAI API returned %s: %s", resp.Status, string(responseBytes))
	}
	if responseBody != nil && len(responseBytes) > 0 {
		if err := json.Unmarshal(responseBytes, responseBody); err != nil {
			return resp.StatusCode, fmt.Errorf("error parsing response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func cachedProjectModelPermissions(ctx context.Context, c *OpenAIClient, projectID string) (*ProjectModelPermissionsAPI, int, error) {
	key := projectSettingsIdentityKey(c, projectID, "model_permissions")
	projectSettingsCache.Lock()
	defer projectSettingsCache.Unlock()

	if cached, ok := projectSettingsCache.modelPermissions[key]; ok && time.Now().Before(cached.ExpiresAt) {
		value := cached.Value
		value.ModelIDs = append([]string(nil), value.ModelIDs...)
		return &value, http.StatusOK, nil
	}
	if cached, ok := readProjectSettingsFileCache[ProjectModelPermissionsAPI]("model-permissions", key); ok {
		projectSettingsCache.modelPermissions[key] = cached
		value := cached.Value
		value.ModelIDs = append([]string(nil), value.ModelIDs...)
		return &value, http.StatusOK, nil
	}

	var value ProjectModelPermissionsAPI
	status, err := projectSettingsRequest(ctx, c, http.MethodGet, "/v1/organization/projects/"+projectID+"/model_permissions", nil, &value)
	if err != nil {
		return nil, status, err
	}
	entry := projectSettingsCacheEntry[ProjectModelPermissionsAPI]{Value: value, ExpiresAt: time.Now().Add(projectSettingsCacheTTL)}
	projectSettingsCache.modelPermissions[key] = entry
	_ = writeProjectSettingsFileCache("model-permissions", key, entry)
	value.ModelIDs = append([]string(nil), value.ModelIDs...)
	return &value, status, nil
}

func cachedProjectSpendAlert(ctx context.Context, c *OpenAIClient, projectID, alertID string) (*ProjectSpendAlertAPI, int, error) {
	key := projectSettingsIdentityKey(c, spendAlertCacheKey(projectID, alertID), "spend_alert")
	projectSettingsCache.Lock()
	defer projectSettingsCache.Unlock()

	if cached, ok := projectSettingsCache.spendAlerts[key]; ok && time.Now().Before(cached.ExpiresAt) {
		value := cloneProjectSpendAlert(cached.Value)
		return &value, http.StatusOK, nil
	}
	if cached, ok := readProjectSettingsFileCache[ProjectSpendAlertAPI]("spend-alerts", key); ok {
		projectSettingsCache.spendAlerts[key] = cached
		value := cloneProjectSpendAlert(cached.Value)
		return &value, http.StatusOK, nil
	}

	var value ProjectSpendAlertAPI
	status, err := projectSettingsRequest(ctx, c, http.MethodGet, "/v1/organization/projects/"+projectID+"/spend_alerts/"+alertID, nil, &value)
	if err != nil {
		return nil, status, err
	}
	entry := projectSettingsCacheEntry[ProjectSpendAlertAPI]{Value: cloneProjectSpendAlert(value), ExpiresAt: time.Now().Add(projectSettingsCacheTTL)}
	projectSettingsCache.spendAlerts[key] = entry
	_ = writeProjectSettingsFileCache("spend-alerts", key, entry)
	value = cloneProjectSpendAlert(value)
	return &value, status, nil
}

func projectSettingsCachePath(kind, key string) string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "terraform-provider-openai", "project-settings", kind, key+".json")
}

func readProjectSettingsFileCache[T any](kind, key string) (projectSettingsCacheEntry[T], bool) {
	path := projectSettingsCachePath(kind, key)
	body, err := os.ReadFile(path)
	if err != nil {
		return projectSettingsCacheEntry[T]{}, false
	}
	var entry projectSettingsCacheEntry[T]
	if err := json.Unmarshal(body, &entry); err != nil || !time.Now().Before(entry.ExpiresAt) {
		_ = os.Remove(path)
		return projectSettingsCacheEntry[T]{}, false
	}
	return entry, true
}

func writeProjectSettingsFileCache[T any](kind, key string, entry projectSettingsCacheEntry[T]) error {
	path := projectSettingsCachePath(kind, key)
	body, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0600)
}

func cloneProjectSpendAlert(value ProjectSpendAlertAPI) ProjectSpendAlertAPI {
	value.NotificationChannel.Recipients = append([]string(nil), value.NotificationChannel.Recipients...)
	if value.NotificationChannel.SubjectPrefix != nil {
		subjectPrefix := *value.NotificationChannel.SubjectPrefix
		value.NotificationChannel.SubjectPrefix = &subjectPrefix
	}
	return value
}
