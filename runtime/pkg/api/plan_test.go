package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/quarkloop/runtime/pkg/plan"
)

func TestPlanGetReturnsCurrentPlanState(t *testing.T) {
	app := fiber.New()
	NewPlanHandler(plan.New()).RegisterRoutes(app.Group("/v1/plan"))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/plan", nil))
	if err != nil {
		t.Fatalf("plan request: %v", err)
	}
	defer resp.Body.Close()

	var payload planResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode plan response: %v", err)
	}
	if payload.Status != "idle" || !payload.Complete || payload.Summary != "No active work." {
		t.Fatalf("unexpected plan response: %+v", payload)
	}
}
