package otp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("OTP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OTP_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	schema := "otp_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(12)
	t.Cleanup(func() {
		sqlDB.Close()
		root.Exec("DROP SCHEMA " + schema + " CASCADE")
		raw, _ := root.DB()
		raw.Close()
	})
	repo := NewRepository(db)
	if err = repo.Migrate(); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	return NewService(repo, cfg, NewProvider(cfg, nil)), db
}
func projectFixture(t *testing.T, s *Service) (Project, string) {
	t.Helper()
	p, key, err := s.CreateProject(context.Background(), NewProject{Name: "Test CRM"})
	if err != nil {
		t.Fatal(err)
	}
	return p, key
}
func requestFixture(t *testing.T, s *Service, p Project) (View, string) {
	t.Helper()
	v, err := s.Request(context.Background(), p, Request{"+919999999999", "login", "attempt-1", true}, uuid.NewString(), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	worked, err := s.ProcessOne(context.Background())
	if err != nil || !worked {
		t.Fatalf("worker: %v", err)
	}
	var j Job
	if err = s.repo.db.First(&j, "challenge_id = ?", v.ChallengeID).Error; err != nil {
		t.Fatal(err)
	}
	code, err := s.open(j.Payload, "code:"+v.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	return v, code
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
func TestIntegrationSingleUseConcurrentAndIsolation(t *testing.T) {
	s, _ := fixture(t)
	p, _ := projectFixture(t, s)
	other, _ := projectFixture(t, s)
	v, code := requestFixture(t, s, p)
	input := Verify{v.ChallengeID, code, "login", "attempt-1"}
	_, err := s.Verify(context.Background(), other, input, "ip")
	requireCode(t, err, "INVALID_OTP")
	_, err = s.Status(context.Background(), other, v.ChallengeID)
	requireCode(t, err, "NOT_FOUND")
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.Verify(context.Background(), p, input, "ip")
			if err == nil && out.Verified {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected exactly one verification, got %d", successes.Load())
	}
	var job Job
	s.repo.db.First(&job, "challenge_id = ?", v.ChallengeID)
	if job.Payload != "" {
		t.Fatal("verified code retained")
	}
}
func TestIntegrationAttemptsExpiryAndBinding(t *testing.T) {
	for _, kind := range []string{"attempts", "expiry", "reference"} {
		t.Run(kind, func(t *testing.T) {
			s, db := fixture(t)
			p, _ := projectFixture(t, s)
			v, code := requestFixture(t, s, p)
			input := Verify{v.ChallengeID, code, "login", "attempt-1"}
			switch kind {
			case "attempts":
				input.Code = "000000"
				if input.Code == code {
					input.Code = "999999"
				}
				for i := 0; i < 5; i++ {
					_, err := s.Verify(context.Background(), p, input, "ip")
					requireCode(t, err, "INVALID_OTP")
				}
				input.Code = code
			case "expiry":
				db.Model(&Challenge{}).Where("id = ?", v.ChallengeID).Update("expires_at", time.Now().Add(-time.Second))
			case "reference":
				input.Reference = "different-attempt"
			}
			_, err := s.Verify(context.Background(), p, input, "ip")
			requireCode(t, err, "INVALID_OTP")
			if kind == "attempts" {
				var c Challenge
				db.First(&c, "id = ?", v.ChallengeID)
				if c.State != "LOCKED" || c.Attempts != 5 {
					t.Fatal("attempts not persisted")
				}
			}
		})
	}
}
func TestIntegrationIdempotencyResendAndCooldown(t *testing.T) {
	s, db := fixture(t)
	p, _ := projectFixture(t, s)
	ctx := context.Background()
	input := Request{"+919999999999", "login", "attempt-1", true}
	key := uuid.NewString()
	v, err := s.Request(ctx, p, input, key, "ip")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Request(ctx, p, input, key, "ip")
	if err != nil || again.ChallengeID != v.ChallengeID {
		t.Fatal("idempotency failed")
	}
	input.Reference = "changed"
	_, err = s.Request(ctx, p, input, key, "ip")
	requireCode(t, err, "IDEMPOTENCY_CONFLICT")
	input.Reference = "attempt-1"
	_, err = s.Request(ctx, p, input, uuid.NewString(), "ip")
	requireCode(t, err, "COOLDOWN")
	db.Model(&Challenge{}).Where("id = ?", v.ChallengeID).Update("created_at", time.Now().Add(-61*time.Second))
	resendKey := uuid.NewString()
	next, err := s.Resend(ctx, p, v.ChallengeID, resendKey, "ip", true)
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.Resend(ctx, p, v.ChallengeID, resendKey, "ip", true)
	if err != nil || same.ChallengeID != next.ChallengeID {
		t.Fatal("resend not idempotent")
	}
	var old Challenge
	db.First(&old, "id = ?", v.ChallengeID)
	if old.State != "SUPERSEDED" || old.Digest != "" {
		t.Fatal("old code not invalidated")
	}
	var count int64
	db.Model(&Job{}).Where("state = ?", "QUEUED").Count(&count)
	if count != 1 {
		t.Fatalf("expected one live job, got %d", count)
	}
}
func TestIntegrationRotationPauseAndLimits(t *testing.T) {
	s, db := fixture(t)
	p, key := projectFixture(t, s)
	ctx := context.Background()
	newKey, err := s.Rotate(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Authenticate(ctx, key)
	requireCode(t, err, "UNAUTHORIZED")
	fresh, err := s.Authenticate(ctx, newKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Request(ctx, p, Request{"+919999999999", "login", "ref", true}, uuid.NewString(), "ip")
	requireCode(t, err, "UNAUTHORIZED")
	v, _ := requestFixture(t, s, fresh)
	if err = s.Enable(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	_, err = s.Authenticate(ctx, newKey)
	requireCode(t, err, "UNAUTHORIZED")
	var c Challenge
	db.First(&c, "id = ?", v.ChallengeID)
	if c.State != "CANCELLED" {
		t.Fatal("pause did not invalidate challenge")
	}
	if err = s.Enable(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	db.Model(&Project{}).Where("id = ?", p.ID).Update("daily_limit", 1)
	_, err = s.Request(ctx, fresh, Request{"+918888888888", "login", "ref", true}, uuid.NewString(), "ip")
	requireCode(t, err, "RATE_LIMITED")
}

type senderFunc func(context.Context, Delivery) SendResult

func (f senderFunc) Send(ctx context.Context, d Delivery) SendResult { return f(ctx, d) }
func TestIntegrationQueueConcurrencyAndRecovery(t *testing.T) {
	s, db := fixture(t)
	p, _ := projectFixture(t, s)
	var calls atomic.Int32
	s.sender = senderFunc(func(context.Context, Delivery) SendResult {
		calls.Add(1)
		return SendResult{State: "UNKNOWN", ErrorCode: "TIMEOUT"}
	})
	v, err := s.Request(context.Background(), p, Request{"+919999999999", "login", "ref", true}, uuid.NewString(), "ip")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ProcessOne(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("job sent more than once")
	}
	s.ProcessOne(context.Background())
	if calls.Load() != 1 {
		t.Fatal("ambiguous send retried")
	}
	db.Model(&Job{}).Where("challenge_id = ?", v.ChallengeID).Updates(map[string]interface{}{"state": "SENDING", "started_at": time.Now().Add(-3 * time.Minute), "payload": "encrypted"})
	if err = s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var j Job
	db.First(&j, "challenge_id = ?", v.ChallengeID)
	if j.State != "UNKNOWN" || j.Payload != "" {
		t.Fatal("interrupted worker recovery failed")
	}
}
func TestIntegrationCleanupAndExpiredQueue(t *testing.T) {
	s, db := fixture(t)
	p, _ := projectFixture(t, s)
	var calls atomic.Int32
	s.sender = senderFunc(func(context.Context, Delivery) SendResult { calls.Add(1); return SendResult{State: "ACCEPTED"} })
	v, err := s.Request(context.Background(), p, Request{"+919999999999", "login", "ref", true}, uuid.NewString(), "ip")
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&Challenge{}).Where("id = ?", v.ChallengeID).Update("expires_at", time.Now().Add(-2*time.Hour))
	if _, err = s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("expired code sent")
	}
	if err = s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var c Challenge
	db.First(&c, "id = ?", v.ChallengeID)
	if c.PhoneCipher != "" || c.Digest != "" || c.State != "EXPIRED" {
		t.Fatal("expired secrets retained")
	}
	db.Model(&Challenge{}).Where("id = ?", v.ChallengeID).Update("created_at", time.Now().Add(-31*24*time.Hour))
	if err = s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&Challenge{}).Count(&count)
	if count != 0 {
		t.Fatal("retention failed")
	}
}
func TestIntegrationReceiptsBeforeSendAndOutOfOrder(t *testing.T) {
	s, _ := fixture(t)
	p, _ := projectFixture(t, s)
	s.sender = senderFunc(func(ctx context.Context, d Delivery) SendResult {
		if err := s.RecordReceipt(ctx, "wamid.early", "read"); err != nil {
			t.Fatal(err)
		}
		return SendResult{State: "ACCEPTED", ProviderID: "wamid.early"}
	})
	v, err := s.Request(context.Background(), p, Request{"+919999999999", "login", "ref", true}, uuid.NewString(), "ip")
	if err != nil {
		t.Fatal(err)
	}
	s.repo.db.Model(&Job{}).Where("challenge_id = ?", v.ChallengeID).Update("provider", "meta")
	if _, err = s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.RecordReceipt(context.Background(), "wamid.early", "delivered")
	s.RecordReceipt(context.Background(), "wamid.early", "failed")
	if err = s.ReconcileReceipts(context.Background()); err != nil {
		t.Fatal(err)
	}
	var j Job
	s.repo.db.First(&j, "challenge_id = ?", v.ChallengeID)
	if j.State != "READ" {
		t.Fatalf("receipt regressed: %s", j.State)
	}
}
func httpCall(r http.Handler, method, path, key string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestIntegrationHTTPContract(t *testing.T) {
	s, _ := fixture(t)
	p, key := projectFixture(t, s)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s.Register(r)
	for _, path := range []string{"/otp", "/otp/app.js", "/otp/style.css"} {
		w := httpCall(r, "GET", path, "", nil, nil)
		if w.Code != 200 || w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("console unavailable")
		}
	}
	w := httpCall(r, "GET", "/otp/admin/projects", "bad", nil, nil)
	if w.Code != 401 {
		t.Fatal("admin exposed")
	}
	w = httpCall(r, "GET", "/otp/admin/projects", s.cfg.AdminToken, nil, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), p.KeyHash) {
		t.Fatal("project key leaked")
	}
	w = httpCall(r, "POST", "/v1/otp/request", key, map[string]interface{}{"phone": "+919999999999", "purpose": "login", "reference": "ref", "consent": true, "extra": true}, nil)
	if w.Code != 400 {
		t.Fatal("unknown fields accepted")
	}
	request := Request{"+919999999999", "login", "ref", true}
	w = httpCall(r, "POST", "/v1/otp/request", key, request, map[string]string{"Idempotency-Key": uuid.NewString()})
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var v View
	json.Unmarshal(w.Body.Bytes(), &v)
	if _, err := s.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	w = httpCall(r, "POST", "/otp/admin/sandbox/"+v.ChallengeID+"/code", key, nil, nil)
	if w.Code != 401 {
		t.Fatal("project can reveal code")
	}
	w = httpCall(r, "POST", "/otp/admin/sandbox/"+v.ChallengeID+"/code", s.cfg.AdminToken, nil, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Code string `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	w = httpCall(r, "POST", "/v1/otp/verify", key, Verify{v.ChallengeID, result.Code, "login", "ref"}, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"sandbox":true`) {
		t.Fatal("sandbox verification indistinguishable")
	}
	w = httpCall(r, "GET", "/otp/admin/activity", s.cfg.AdminToken, nil, nil)
	if strings.Contains(w.Body.String(), result.Code) || strings.Contains(w.Body.String(), request.Phone) {
		t.Fatal("activity leaked secrets")
	}
}
func TestIntegrationWebhookAuthentication(t *testing.T) {
	s, _ := fixture(t)
	s.cfg.MetaAppSecret = "webhook-secret"
	s.cfg.MetaPhoneID = "123"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s.Register(r)
	body := map[string]interface{}{"entry": []interface{}{map[string]interface{}{"changes": []interface{}{map[string]interface{}{"value": map[string]interface{}{"metadata": map[string]string{"phone_number_id": "123"}, "statuses": []interface{}{map[string]string{"id": "wamid.test", "status": "read"}}}}}}}}
	w := httpCall(r, "POST", "/otp/webhooks/meta", "", body, nil)
	if w.Code != 401 {
		t.Fatal("unsigned webhook accepted")
	}
	raw, _ := json.Marshal(body)
	mac := hmac.New(sha256.New, []byte(s.cfg.MetaAppSecret))
	mac.Write(raw)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	w = httpCall(r, "POST", "/otp/webhooks/meta", "", body, map[string]string{"X-Hub-Signature-256": signature})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var receipt Receipt
	if err := s.repo.db.First(&receipt, "provider_id = ?", "wamid.test").Error; err != nil || receipt.State != "READ" {
		t.Fatal("signed webhook not recorded")
	}
}
func TestIntegrationBurstRequests(t *testing.T) {
	s, _ := fixture(t)
	p, _ := projectFixture(t, s)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.Request(context.Background(), p, Request{fmt.Sprintf("+91999999%04d", i), "login", fmt.Sprintf("attempt-%d", i), true}, uuid.NewString(), "ip")
			if err != nil {
				t.Error(err)
			} else {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 12 {
		t.Fatal("burst failed")
	}
}

func TestIntegrationRetryBoundAndIdempotentBurst(t *testing.T) {
	s, db := fixture(t)
	p, _ := projectFixture(t, s)
	ctx := context.Background()
	input := Request{"+919999999999", "login", "ref", true}
	key := uuid.NewString()
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.Request(ctx, p, input, key, "ip")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- v.ChallengeID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("concurrent idempotency created duplicates")
		}
	}
	var count int64
	db.Model(&Job{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate jobs")
	}
	var calls int
	s.sender = senderFunc(func(context.Context, Delivery) SendResult {
		calls++
		return SendResult{State: "QUEUED", Retry: true, ErrorCode: "PROVIDER_RATE_LIMIT"}
	})
	for i := 0; i < 3; i++ {
		db.Model(&Job{}).Where("challenge_id = ?", first).Update("available_at", time.Now().Add(-time.Second))
		if _, err := s.ProcessOne(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var job Job
	db.First(&job, "challenge_id = ?", first)
	if calls != 3 || job.State != "FAILED" || job.Payload != "" {
		t.Fatal("retry budget not enforced")
	}
	if _, err := s.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal("failed job resent")
	}
}

func TestIntegrationValidationAndSecretStorage(t *testing.T) {
	s, db := fixture(t)
	p, _ := projectFixture(t, s)
	input := Request{"+919999999999", "login", "ref", false}
	_, err := s.Request(context.Background(), p, input, uuid.NewString(), "ip")
	requireCode(t, err, "INVALID_REQUEST")
	input.Consent = true
	input.Phone = "919999999999"
	_, err = s.Request(context.Background(), p, input, uuid.NewString(), "ip")
	requireCode(t, err, "INVALID_REQUEST")
	v, code := requestFixture(t, s, p)
	var c Challenge
	db.First(&c, "id = ?", v.ChallengeID)
	if strings.Contains(c.PhoneCipher, "919999999999") || strings.Contains(c.Digest, code) {
		t.Fatal("plaintext secrets stored")
	}
	s.cfg.AllowSandbox = false
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s.Register(r)
	w := httpCall(r, "POST", "/otp/admin/sandbox/"+v.ChallengeID+"/code", s.cfg.AdminToken, nil, nil)
	if w.Code != 404 {
		t.Fatal("disabled sandbox exposed code")
	}
}
