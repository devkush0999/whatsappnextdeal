// Package otptransport delivers NextDeal-owned codes without issuing sessions.
package otptransport

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type SendFunc func(context.Context, string, string, string, string) (string, error)
type Config struct {
	Enabled         bool
	Key, InstanceID string
}

func LoadConfig() (Config, error) {
	c := Config{Enabled: os.Getenv("NEXTDEAL_OTP_TRANSPORT_ENABLED") == "true", Key: os.Getenv("NEXTDEAL_OTP_TRANSPORT_KEY"), InstanceID: os.Getenv("NEXTDEAL_OTP_INSTANCE_ID")}
	if c.Enabled && (len(c.Key) < 32 || c.InstanceID == "") {
		return c, errors.New("configure NextDeal OTP transport key (32+ characters) and instance ID")
	}
	return c, nil
}

type request struct {
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	RequestID string `json:"requestId"`
}

var phonePattern = regexp.MustCompile(`^91[6-9][0-9]{9}$`)
var codePattern = regexp.MustCompile(`^[0-9]{4}$`)
var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Single-replica transport guard, not the login attempt limiter. Login verification,
// expiry and persistent abuse controls remain owned by NextDeal's backend.
type Handler struct {
	cfg   Config
	send  SendFunc
	mu    sync.Mutex
	last  map[string]time.Time
	ids   map[string]time.Time
	day   string
	count int
}

func Register(r *gin.Engine, c Config, send SendFunc) {
	if !c.Enabled {
		return
	}
	h := &Handler{cfg: c, send: send, last: map[string]time.Time{}, ids: map[string]time.Time{}}
	r.POST("/internal/nextdeal/otp", h.deliver)
}
func (h *Handler) deliver(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Access-Control-Allow-Origin", "")
	c.Header("Access-Control-Allow-Credentials", "")
	want := sha256.Sum256([]byte(h.cfg.Key))
	got := sha256.Sum256([]byte(c.GetHeader("X-NextDeal-OTP-Key")))
	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		c.AbortWithStatusJSON(401, gin.H{"error": "UNAUTHORIZED"})
		return
	}
	var b request
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		c.AbortWithStatusJSON(400, gin.H{"error": "INVALID_REQUEST"})
		return
	}
	var extra interface{}
	if dec.Decode(&extra) != io.EOF || !phonePattern.MatchString(b.Phone) || !codePattern.MatchString(b.Code) || !idPattern.MatchString(b.RequestID) {
		c.AbortWithStatusJSON(400, gin.H{"error": "INVALID_REQUEST"})
		return
	}
	now := time.Now()
	h.mu.Lock()
	for k, t := range h.last {
		if now.Sub(t) >= time.Hour {
			delete(h.last, k)
		}
	}
	for k, t := range h.ids {
		if now.Sub(t) >= time.Hour {
			delete(h.ids, k)
		}
	}
	if h.day != now.UTC().Format("2006-01-02") {
		h.day = now.UTC().Format("2006-01-02")
		h.count = 0
	}
	if _, seen := h.ids[b.RequestID]; seen {
		h.mu.Unlock()
		c.AbortWithStatusJSON(409, gin.H{"error": "DUPLICATE_REQUEST"})
		return
	}
	if now.Sub(h.last[b.Phone]) < 30*time.Second || h.count >= 300 {
		h.mu.Unlock()
		c.Header("Retry-After", "30")
		c.AbortWithStatusJSON(429, gin.H{"error": "DELIVERY_LIMIT"})
		return
	}
	h.last[b.Phone] = now
	h.ids[b.RequestID] = now
	h.count++
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	message := fmt.Sprintf("%s is your NextDeal login verification code. It expires in 10 minutes. Do not share this code. If you did not request it, ignore this message.", b.Code)
	id, err := h.send(ctx, h.cfg.InstanceID, b.Phone, message, b.RequestID)
	if err != nil || id == "" {
		c.AbortWithStatusJSON(503, gin.H{"error": "DELIVERY_UNCONFIRMED"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "ACCEPTED", "requestId": b.RequestID})
}
