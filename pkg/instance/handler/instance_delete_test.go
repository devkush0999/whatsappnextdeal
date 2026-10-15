package instance_handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	"github.com/gin-gonic/gin"
)

type deleteService struct {
	instance_service.InstanceService
	deletedID string
}

func (s *deleteService) Delete(id string) error {
	s.deletedID = id
	return nil
}

func TestDeleteProtectsActiveOTPSender(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("NEXTDEAL_OTP_INSTANCE_ID", "active-sender")
	for _, tc := range []struct {
		name, id string
		want     int
	}{
		{"active sender", "active-sender", http.StatusConflict},
		{"other sender", "old-sender", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &deleteService{}
			router := gin.New()
			router.DELETE("/instance/delete/:instanceId", (&instanceHandler{instanceService: service}).Delete)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/instance/delete/"+tc.id, nil))
			if response.Code != tc.want {
				t.Fatalf("got %d, want %d", response.Code, tc.want)
			}
			if tc.want == http.StatusConflict && service.deletedID != "" {
				t.Fatal("protected sender reached delete service")
			}
			if tc.want == http.StatusOK && service.deletedID != tc.id {
				t.Fatal("different sender reached delete service")
			}
		})
	}
}
