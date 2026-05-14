package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/runtime/pkg/activity"
)

func TestActivityListReturnsMappedRecords(t *testing.T) {
	store := activity.NewStore(10)
	store.Add("s1", "message.user", map[string]any{"content_length": 5})

	app := fiber.New()
	NewActivityHandler(store).RegisterRoutes(app.Group("/v1/activity"))

	req := httptest.NewRequest("GET", "/v1/activity?limit=1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("activity request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var records []activityResponse
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(records) != 1 || records[0].Type != "message.user" || records[0].SessionID != "s1" {
		t.Fatalf("unexpected records: %+v", records)
	}
}
