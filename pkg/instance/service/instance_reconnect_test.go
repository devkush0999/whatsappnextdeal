package instance_service

import (
	"errors"
	"testing"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
)

type reconnectRecorder struct {
	whatsmeow_service.WhatsmeowService
	ids []string
	err error
}

func (r *reconnectRecorder) ReconnectClient(id string) error {
	r.ids = append(r.ids, id)
	return r.err
}

func TestReconnectRecoversDisconnectedAndMissingClients(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client *whatsmeow.Client
	}{
		{name: "missing client"},
		{name: "existing disconnected client", client: &whatsmeow.Client{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &reconnectRecorder{}
			service := instances{
				whatsmeowService: recorder,
				clientPointer:    map[string]*whatsmeow.Client{"sender-a": tc.client},
			}
			if err := service.Reconnect(&instance_model.Instance{Id: "sender-a"}); err != nil {
				t.Fatalf("recovery rejected: %v", err)
			}
			if len(recorder.ids) != 1 || recorder.ids[0] != "sender-a" {
				t.Fatalf("recovery must target the authenticated sender exactly once: %v", recorder.ids)
			}
		})
	}
}

func TestReconnectRejectsMissingIdentity(t *testing.T) {
	for _, instance := range []*instance_model.Instance{nil, {}, {Id: "  "}} {
		recorder := &reconnectRecorder{}
		service := instances{whatsmeowService: recorder}
		if service.Reconnect(instance) == nil {
			t.Fatal("missing sender identity must fail")
		}
		if len(recorder.ids) != 0 {
			t.Fatal("missing identity reached recovery")
		}
	}
}

func TestReconnectPreservesRecoveryError(t *testing.T) {
	want := errors.New("synthetic recovery failure")
	recorder := &reconnectRecorder{err: want}
	service := instances{whatsmeowService: recorder}
	if err := service.Reconnect(&instance_model.Instance{Id: "sender-a"}); !errors.Is(err, want) {
		t.Fatalf("got %v, want original recovery error", err)
	}
}
