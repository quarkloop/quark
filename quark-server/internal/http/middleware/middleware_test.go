package middleware

import (
        "context"
        "net/http/httptest"
        "testing"

        "github.com/gofiber/fiber/v2"
        "github.com/google/uuid"
        "go.uber.org/zap"
)

func TestRequestID_GeneratesUUID(t *testing.T) {
        app := fiber.New()
        app.Use(RequestID())
        app.Get("/", func(c *fiber.Ctx) error {
                return c.SendString(c.Locals("requestId").(string))
        })

        resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
        if err != nil {
                t.Fatal(err)
        }
        if resp.StatusCode != 200 {
                t.Errorf("status = %d, want 200", resp.StatusCode)
        }
        // X-Request-ID response header should be set
        if id := resp.Header.Get("X-Request-ID"); id == "" {
                t.Error("X-Request-ID header missing")
        } else if _, err := uuid.Parse(id); err != nil {
                t.Errorf("X-Request-ID = %q, not a valid UUID", id)
        }
}

func TestRequestID_PreservesClientHeader(t *testing.T) {
        app := fiber.New()
        app.Use(RequestID())
        app.Get("/", func(c *fiber.Ctx) error {
                return c.SendString(c.Locals("requestId").(string))
        })

        req := httptest.NewRequest("GET", "/", nil)
        req.Header.Set("X-Request-ID", "client-provided-id-12345")
        resp, err := app.Test(req)
        if err != nil {
                t.Fatal(err)
        }
        if id := resp.Header.Get("X-Request-ID"); id != "client-provided-id-12345" {
                t.Errorf("X-Request-ID = %q, want client-provided-id-12345", id)
        }
}

func TestGetRequestID(t *testing.T) {
        // Empty context returns ""
        if got := GetRequestID(context.Background()); got != "" {
                t.Errorf("GetRequestID(empty ctx) = %q, want empty", got)
        }

        // Context with a request ID returns it
        ctx := context.WithValue(context.Background(), RequestIDKey, "abc-123")
        if got := GetRequestID(ctx); got != "abc-123" {
                t.Errorf("GetRequestID = %q, want abc-123", got)
        }

        // Context with a non-string value returns ""
        ctx = context.WithValue(context.Background(), RequestIDKey, 42)
        if got := GetRequestID(ctx); got != "" {
                t.Errorf("GetRequestID(non-string) = %q, want empty", got)
        }
}

func TestRecoverer_CatchesPanic(t *testing.T) {
        app := fiber.New()
        app.Use(Recoverer(zap.NewNop()))
        app.Get("/panic", func(c *fiber.Ctx) error {
                panic("test panic")
        })

        resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
        if err != nil {
                t.Fatal(err)
        }
        if resp.StatusCode != 500 {
                t.Errorf("status = %d, want 500", resp.StatusCode)
        }
}

func TestRecoverer_NoPanic(t *testing.T) {
        app := fiber.New()
        app.Use(Recoverer(zap.NewNop()))
        app.Get("/", func(c *fiber.Ctx) error {
                return c.SendString("ok")
        })

        resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
        if err != nil {
                t.Fatal(err)
        }
        if resp.StatusCode != 200 {
                t.Errorf("status = %d, want 200", resp.StatusCode)
        }
}

func TestCORS_SetsHeaders(t *testing.T) {
        app := fiber.New()
        app.Use(CORS())
        app.Get("/", func(c *fiber.Ctx) error {
                return c.SendString("ok")
        })

        resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
        if err != nil {
                t.Fatal(err)
        }
        if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
                t.Error("Access-Control-Allow-Origin missing or wrong")
        }
        if resp.Header.Get("Access-Control-Allow-Methods") == "" {
                t.Error("Access-Control-Allow-Methods missing")
        }
}

func TestCORS_OptionsReturns204(t *testing.T) {
        app := fiber.New()
        app.Use(CORS())
        app.Get("/", func(c *fiber.Ctx) error {
                return c.SendString("ok")
        })

        req := httptest.NewRequest("GET", "/", nil)
        req.Method = "OPTIONS"
        resp, err := app.Test(req)
        if err != nil {
                t.Fatal(err)
        }
        if resp.StatusCode != 204 {
                t.Errorf("OPTIONS status = %d, want 204", resp.StatusCode)
        }
}
