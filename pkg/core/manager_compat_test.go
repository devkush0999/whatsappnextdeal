package core_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/evolution-foundation/evolution-go/pkg/core"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	auth_middleware "github.com/evolution-foundation/evolution-go/pkg/middleware"
	"github.com/gin-gonic/gin"
)

type blockedTransport struct{ requests atomic.Int64 }

func (b *blockedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	b.requests.Add(1)
	return nil, errors.New("outbound network disabled for test")
}

func TestManagerCompatibilityWorksWithoutNetworkOrLicense(t *testing.T) {
	transport := &blockedTransport{}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })

	router := gin.New()
	core.RegisterManagerCompatibilityRoutes(router)
	for _, path := range []string{
		"/license/status",
		"/license/register?redirect_uri=https://example.invalid/callback",
		"/license/activate?code=obsolete-code",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("apikey", "local-test-secret")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["status"] != "active" || body["mode"] != "standalone" || body["license_required"] != false {
				t.Fatalf("unexpected compatibility response: %v", body)
			}
			for _, key := range []string{"api_key", "register_url", "instance_id"} {
				if _, exists := body[key]; exists {
					t.Errorf("response exposes unnecessary field %q", key)
				}
			}
			if strings.Contains(response.Body.String(), "local-test-secret") || response.Header().Get("Location") != "" {
				t.Fatal("response leaked a credential or redirected to registration")
			}
		})
	}
	if calls := transport.requests.Load(); calls != 0 {
		t.Fatalf("made %d outbound requests", calls)
	}
}

type instanceLookup struct {
	instance_service.InstanceService
}

func (instanceLookup) GetInstanceByToken(token string) (*instance_model.Instance, error) {
	if token != "instance-test-key" {
		return nil, errors.New("unknown token")
	}
	return &instance_model.Instance{Id: "test-instance"}, nil
}

func TestStandaloneCompatibilityDoesNotAuthorizeAPIRequests(t *testing.T) {
	router := gin.New()
	core.RegisterManagerCompatibilityRoutes(router)
	auth := auth_middleware.NewMiddleware(&config.Config{GlobalApiKey: "admin-test-key"}, instanceLookup{})
	router.GET("/admin", auth.AuthAdmin, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/instance", auth.Auth, func(c *gin.Context) {
		instance := c.MustGet("instance").(*instance_model.Instance)
		if instance.Id != "test-instance" {
			t.Errorf("unexpected authenticated instance %q", instance.Id)
		}
		c.Status(http.StatusNoContent)
	})

	for _, tc := range []struct {
		path, token string
		want        int
	}{
		{"/admin", "", http.StatusUnauthorized},
		{"/admin", "wrong", http.StatusUnauthorized},
		{"/admin", "instance-test-key", http.StatusUnauthorized},
		{"/admin", "admin-test-key", http.StatusNoContent},
		{"/instance", "", http.StatusUnauthorized},
		{"/instance", "wrong", http.StatusUnauthorized},
		{"/instance", "admin-test-key", http.StatusUnauthorized},
		{"/instance", "instance-test-key", http.StatusNoContent},
	} {
		t.Run(tc.path+"/"+tc.token, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("apikey", tc.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
		})
	}
}
