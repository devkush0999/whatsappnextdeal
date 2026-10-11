package otpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer otp_test" {
			t.Error("missing auth")
		}
		if r.URL.Path == "/v1/otp/request" {
			if r.Header.Get("Idempotency-Key") != "request-123" {
				t.Error("missing idempotency")
			}
			w.WriteHeader(202)
			w.Write([]byte(`{"challengeId":"test","status":"ACTIVE","sandbox":true}`))
			return
		}
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"code":"COOLDOWN","message":"Wait","retryAfter":60}}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "otp_test")
	if err != nil {
		t.Fatal(err)
	}
	out, err := client.Request(context.Background(), Request{Consent: true}, "request-123")
	if err != nil || out.ID != "test" || !out.Sandbox {
		t.Fatal("response lost")
	}
	_, err = client.Verify(context.Background(), Verify{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.RetryAfter != 60 {
		t.Fatal("structured error lost")
	}
}
func TestRefusesCredentialRedirect(t *testing.T) {
	var received bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client, _ := New(source.URL, "otp_test")
	client.Status(context.Background(), "id")
	if received {
		t.Fatal("followed credential redirect")
	}
}
