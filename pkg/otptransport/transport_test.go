package otptransport

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeliveryTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := strings.Repeat("k", 32)
	for _, tc := range []struct {
		name, body, key string
		fail            bool
		want            int
		calls           int
	}{
		{"valid", `{"phone":"919000000001","code":"1234","requestId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, key, false, 202, 1},
		{"unauthorized", `{}`, "bad", false, 401, 0},
		{"six-digit rejected", `{"phone":"919000000001","code":"123456","requestId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, key, false, 400, 0},
		{"invalid phone", `{"phone":"group@g.us","code":"1234","requestId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, key, false, 400, 0},
		{"unknown field", `{"phone":"919000000001","code":"1234","requestId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","text":"arbitrary"}`, key, false, 400, 0},
		{"null", `null`, key, false, 400, 0},
		{"uncertain", `{"phone":"919000000001","code":"1234","requestId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, key, true, 503, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			r := gin.New()
			Register(r, Config{true, key, "instance"}, func(ctx context.Context, i, n, text, id string) (string, error) {
				calls++
				if _, ok := ctx.Deadline(); !ok {
					t.Error("missing timeout")
				}
				if i != "instance" || n != "919000000001" || !strings.Contains(text, "1234") {
					t.Error("wrong delivery")
				}
				if tc.fail {
					return "", errors.New("provider secret")
				}
				return "id", nil
			})
			req := httptest.NewRequest("POST", "/internal/nextdeal/otp", strings.NewReader(tc.body))
			req.Header.Set("X-NextDeal-OTP-Key", tc.key)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want || calls != tc.calls {
				t.Fatalf("status %d calls %d", w.Code, calls)
			}
			if strings.Contains(w.Body.String(), "1234") || strings.Contains(w.Body.String(), "provider secret") {
				t.Fatal("secret leaked")
			}
		})
	}
}
func TestDuplicateAndCooldown(t *testing.T) {
	r := gin.New()
	key := strings.Repeat("k", 32)
	calls := 0
	Register(r, Config{true, key, "instance"}, func(context.Context, string, string, string, string) (string, error) { calls++; return "id", nil })
	for i, expected := range []int{202, 409, 429} {
		id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		if i == 2 {
			id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}
		req := httptest.NewRequest("POST", "/internal/nextdeal/otp", strings.NewReader(`{"phone":"919000000001","code":"1234","requestId":"`+id+`"}`))
		req.Header.Set("X-NextDeal-OTP-Key", key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != expected {
			t.Fatalf("got %d want %d", w.Code, expected)
		}
	}
	if calls != 1 {
		t.Fatal("duplicate delivery")
	}
}
func TestDisabledAndConfig(t *testing.T) {
	t.Setenv("NEXTDEAL_OTP_TRANSPORT_ENABLED", "true")
	t.Setenv("NEXTDEAL_OTP_TRANSPORT_KEY", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("invalid config accepted")
	}
	r := gin.New()
	Register(r, Config{}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/internal/nextdeal/otp", nil))
	if w.Code != 404 {
		t.Fatal("disabled route exposed")
	}
}
