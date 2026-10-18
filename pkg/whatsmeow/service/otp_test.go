package whatsmeow_service

import (
	"context"
	"errors"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
	"github.com/evolution-foundation/evolution-go/pkg/otptransport"
	"go.mau.fi/whatsmeow"
	"sync"
	"testing"
)

type otpRepositoryStub struct {
	instance_repository.InstanceRepository
	deleted bool
}

func (r otpRepositoryStub) GetInstanceByID(id string) (*instance_model.Instance, error) {
	if r.deleted {
		return nil, errors.New("missing")
	}
	return &instance_model.Instance{Id: id}, nil
}
func TestOTPRegistrySharedAcrossClientLifecycleCopies(t *testing.T) {
	service := &whatsmeowService{otpClients: &sync.Map{}, instanceRepository: otpRepositoryStub{}}
	// Existing client startup uses value receivers. The registry must remain shared.
	lifecycleCopy := *service
	lifecycleCopy.otpClients.Store("instance", &whatsmeow.Client{})
	_, err := service.SendOTP(context.Background(), "instance", "15555550123", "code", "id")
	if !errors.Is(err, otptransport.ErrSenderUnavailable) {
		t.Fatalf("registered client not visible through original service: %v", err)
	}
	service.instanceRepository = otpRepositoryStub{deleted: true}
	_, err = service.SendOTP(context.Background(), "instance", "15555550123", "code", "id")
	if !errors.Is(err, otptransport.ErrSenderUnavailable) {
		t.Fatal("deleted instance remained sendable")
	}
}
