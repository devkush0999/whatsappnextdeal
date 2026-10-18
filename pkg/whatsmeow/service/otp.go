package whatsmeow_service

import (
	"context"
	"fmt"
	"github.com/evolution-foundation/evolution-go/pkg/otptransport"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// SendOTP uses a synchronized registry, a bounded caller context and a stable
// message ID. No reconnect or resend loop runs on an authentication request.
func (w *whatsmeowService) SendOTP(ctx context.Context, instanceID, number, text, messageID string) (string, error) {
	if _, err := w.instanceRepository.GetInstanceByID(instanceID); err != nil {
		return "", fmt.Errorf("OTP instance unavailable: %w", otptransport.ErrSenderUnavailable)
	}
	value, ok := w.otpClients.Load(instanceID)
	if !ok {
		return "", fmt.Errorf("OTP instance unavailable: %w", otptransport.ErrSenderUnavailable)
	}
	client := value.(*whatsmeow.Client)
	if !client.IsConnected() || !client.IsLoggedIn() {
		return "", fmt.Errorf("OTP instance disconnected: %w", otptransport.ErrSenderUnavailable)
	}
	response, err := client.SendMessage(ctx, types.NewJID(number, types.DefaultUserServer), &waE2E.Message{Conversation: &text}, whatsmeow.SendRequestExtra{ID: messageID})
	if err != nil {
		return "", err
	}
	return response.ID, nil
}
