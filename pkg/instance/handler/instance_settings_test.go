package instance_handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	"github.com/gin-gonic/gin"
)

type settingsService struct {
	instance_service.InstanceService
	reads, writes int
	lastID        string
}

func (s *settingsService) GetAdvancedSettings(id string) (*instance_model.AdvancedSettings, error) {
	s.reads++
	s.lastID = id
	return &instance_model.AdvancedSettings{IgnoreStatus: true}, nil
}
func (s *settingsService) UpdateAdvancedSettings(id string, data *instance_model.AdvancedSettings) error {
	s.writes++
	s.lastID = id
	return nil
}

func TestSenderSettingsOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, tc := range []struct {
			name, path    string
			authenticated bool
			want          int
		}{
			{"own", "sender-a", true, http.StatusOK},
			{"other", "sender-b", true, http.StatusForbidden},
			{"unauthenticated", "sender-a", false, http.StatusUnauthorized},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				service := &settingsService{}
				handler := &instanceHandler{instanceService: service}
				router := gin.New()
				router.Use(func(c *gin.Context) {
					if tc.authenticated {
						c.Set("instance", &instance_model.Instance{Id: "sender-a"})
					}
					c.Next()
				})
				router.GET("/instance/:instanceId/advanced-settings", handler.GetAdvancedSettings)
				router.PUT("/instance/:instanceId/advanced-settings", handler.UpdateAdvancedSettings)
				request := httptest.NewRequest(method, "/instance/"+tc.path+"/advanced-settings", strings.NewReader(`{"ignoreStatus":true}`))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != tc.want {
					t.Fatalf("got status %d, want %d", response.Code, tc.want)
				}
				if tc.want != http.StatusOK && (service.reads != 0 || service.writes != 0) {
					t.Fatal("unauthorized request reached settings service")
				}
				if tc.want == http.StatusOK && service.lastID != "sender-a" {
					t.Fatal("settings operation used wrong sender")
				}
			})
		}
	}
}
