package otp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestMetaPayloadAndResponse(t *testing.T) {
	cfg := testConfig()
	cfg.MetaToken = "test-token"
	cfg.MetaPhoneID = "123"
	cfg.MetaVersion = "v25.0"
	p := NewProvider(cfg, nil)
	p.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "graph.facebook.com" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("invalid destination/auth")
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		template := body["template"].(map[string]interface{})
		components := template["components"].([]interface{})
		if len(components) != 2 || body["to"] != "919999999999" {
			t.Fatal("invalid template payload")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"messages":[{"id":"wamid.test"}]}`)), Header: make(http.Header)}, nil
	})
	out := p.Send(context.Background(), Delivery{Project: Project{Provider: "meta", Template: "test", Language: "en_US"}, Phone: "+919999999999", Code: "123456"})
	if out.State != "ACCEPTED" || out.ProviderID != "wamid.test" {
		t.Fatal(out)
	}
	for _, status := range []int{429, 500, 401} {
		p.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		})
		result := p.Send(context.Background(), Delivery{Project: Project{Provider: "meta"}})
		if status == 429 && !result.Retry {
			t.Fatal("rate limit not retryable")
		}
		if status != 429 && result.Retry {
			t.Fatal("unsafe blind retry")
		}
		if status == 500 && result.State != "UNKNOWN" {
			t.Fatal("ambiguous response not preserved")
		}
	}
}
