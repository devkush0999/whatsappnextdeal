package otp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Delivery struct {
	Project                  Project
	ChallengeID, Phone, Code string
}
type SendResult struct {
	State, ProviderID, ErrorCode string
	Retry                        bool
}
type Sender interface {
	Send(context.Context, Delivery) SendResult
}
type EvolutionSend func(context.Context, string, string, string, string) (string, error)
type Provider struct {
	cfg       Config
	evolution EvolutionSend
	client    *http.Client
}

func NewProvider(cfg Config, evolution EvolutionSend) *Provider {
	return &Provider{cfg: cfg, evolution: evolution, client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (p *Provider) Send(ctx context.Context, d Delivery) SendResult {
	switch d.Project.Provider {
	case "sandbox":
		if p.cfg.AllowSandbox {
			return SendResult{State: "SIMULATED"}
		}
		return SendResult{State: "FAILED", ErrorCode: "PROVIDER_DISABLED"}
	case "evolution":
		if !p.cfg.AllowEvolution || p.evolution == nil {
			return SendResult{State: "FAILED", ErrorCode: "PROVIDER_DISABLED"}
		}
		message := fmt.Sprintf("%s is your %s verification code. It expires in %d minutes. Do not share this code.", d.Code, d.Project.Name, d.Project.TTL/60)
		id, err := p.evolution(ctx, d.Project.InstanceID, strings.TrimPrefix(d.Phone, "+"), message, strings.ReplaceAll(d.ChallengeID, "-", ""))
		if err != nil {
			return SendResult{State: "UNKNOWN", ErrorCode: "GATEWAY_SEND_UNCONFIRMED"}
		}
		return SendResult{State: "ACCEPTED", ProviderID: id}
	case "meta":
		return p.meta(ctx, d)
	default:
		return SendResult{State: "FAILED", ErrorCode: "PROVIDER_DISABLED"}
	}
}
func (p *Provider) meta(ctx context.Context, d Delivery) SendResult {
	if p.cfg.MetaToken == "" {
		return SendResult{State: "FAILED", ErrorCode: "PROVIDER_DISABLED"}
	}
	param := []map[string]string{{"type": "text", "text": d.Code}}
	body := map[string]interface{}{"messaging_product": "whatsapp", "to": strings.TrimPrefix(d.Phone, "+"), "type": "template", "template": map[string]interface{}{"name": d.Project.Template, "language": map[string]string{"code": d.Project.Language}, "components": []interface{}{map[string]interface{}{"type": "body", "parameters": param}, map[string]interface{}{"type": "button", "sub_type": "url", "index": "0", "parameters": param}}}}
	raw, err := json.Marshal(body)
	if err != nil {
		return SendResult{State: "FAILED", ErrorCode: "INVALID_PAYLOAD"}
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+p.cfg.MetaVersion+"/"+p.cfg.MetaPhoneID+"/messages", bytes.NewReader(raw))
	if err != nil {
		return SendResult{State: "FAILED", ErrorCode: "INVALID_CONFIG"}
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.MetaToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return SendResult{State: "UNKNOWN", ErrorCode: "PROVIDER_TIMEOUT"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return SendResult{State: "QUEUED", ErrorCode: "PROVIDER_RATE_LIMIT", Retry: true}
	}
	if resp.StatusCode >= 500 {
		return SendResult{State: "UNKNOWN", ErrorCode: "PROVIDER_UNCONFIRMED"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendResult{State: "FAILED", ErrorCode: fmt.Sprintf("PROVIDER_HTTP_%d", resp.StatusCode)}
	}
	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); err != nil || len(result.Messages) == 0 || result.Messages[0].ID == "" {
		return SendResult{State: "UNKNOWN", ErrorCode: "PROVIDER_INVALID_RESPONSE"}
	}
	return SendResult{State: "ACCEPTED", ProviderID: result.Messages[0].ID}
}
