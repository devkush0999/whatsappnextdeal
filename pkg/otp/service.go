package otp

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"regexp"
	"strings"
	"time"
)

type Error struct {
	Status     int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

func (e *Error) Error() string { return e.Message }
func problem(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

type Service struct {
	repo   *Repository
	cfg    Config
	now    func() time.Time
	sender Sender
}

func NewService(repo *Repository, cfg Config, sender Sender) *Service {
	return &Service{repo: repo, cfg: cfg, now: time.Now, sender: sender}
}
func validID(id string) bool { _, err := uuid.Parse(id); return len(id) == 36 && err == nil }

var purposePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)
var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

type NewProject struct {
	Name             string `json:"name"`
	Provider         string `json:"provider"`
	InstanceID       string `json:"instanceId"`
	Template         string `json:"template"`
	Language         string `json:"language"`
	TTL              int    `json:"ttlSeconds"`
	MaxAttempts      int    `json:"maxAttempts"`
	Cooldown         int    `json:"cooldownSeconds"`
	PhoneHourlyLimit int    `json:"phoneHourlyLimit"`
	DailyLimit       int    `json:"dailyLimit"`
}

func (s *Service) CreateProject(ctx context.Context, in NewProject) (Project, string, error) {
	p := Project{ID: uuid.NewString(), Name: strings.TrimSpace(in.Name), Provider: in.Provider, InstanceID: in.InstanceID, Template: in.Template, Language: in.Language, TTL: in.TTL, MaxAttempts: in.MaxAttempts, Cooldown: in.Cooldown, PhoneHourlyLimit: in.PhoneHourlyLimit, DailyLimit: in.DailyLimit, Enabled: true}
	if p.Provider == "" {
		p.Provider = "sandbox"
	}
	if p.Language == "" {
		p.Language = "en_US"
	}
	if p.TTL == 0 {
		p.TTL = 300
	}
	if p.MaxAttempts == 0 {
		p.MaxAttempts = 5
	}
	if p.Cooldown == 0 {
		p.Cooldown = 60
	}
	if p.PhoneHourlyLimit == 0 {
		p.PhoneHourlyLimit = 5
	}
	if p.DailyLimit == 0 {
		p.DailyLimit = 500
	}
	if len(p.Name) < 1 || len(p.Name) > 80 || p.TTL < 60 || p.TTL > 600 || p.MaxAttempts < 1 || p.MaxAttempts > 10 || p.Cooldown < 30 || p.Cooldown > 300 || p.PhoneHourlyLimit < 1 || p.PhoneHourlyLimit > 10 || p.DailyLimit < 1 || p.DailyLimit > 10000 {
		return p, "", problem(400, "INVALID_PROJECT", "Invalid project name or policy limits")
	}
	switch p.Provider {
	case "sandbox":
		if !s.cfg.AllowSandbox {
			return p, "", problem(400, "PROVIDER_DISABLED", "Sandbox is disabled")
		}
	case "evolution":
		if !s.cfg.AllowEvolution || !validID(p.InstanceID) {
			return p, "", problem(400, "PROVIDER_DISABLED", "Enable Evolution delivery and select a valid instance")
		}
		var count int64
		if err := s.repo.db.WithContext(ctx).Table("instances").Where("id = ?", p.InstanceID).Count(&count).Error; err != nil {
			return p, "", err
		}
		if count == 0 {
			return p, "", problem(400, "INVALID_INSTANCE", "Instance does not exist")
		}
	case "meta":
		if s.cfg.MetaToken == "" || !regexp.MustCompile(`^[a-z0-9_]{1,128}$`).MatchString(p.Template) || !regexp.MustCompile(`^[a-z]{2,3}(_[A-Z]{2})?$`).MatchString(p.Language) {
			return p, "", problem(400, "PROVIDER_DISABLED", "Configure Meta and supply an approved authentication template")
		}
	default:
		return p, "", problem(400, "INVALID_PROVIDER", "Unknown provider")
	}
	key := "otp_" + p.ID + "." + randomSecret()
	p.KeyHash = s.digest("key", key)
	err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		return audit(tx, p.ID, "project.created", p.ID)
	})
	return p, key, err
}
func (s *Service) Authenticate(ctx context.Context, key string) (Project, error) {
	var p Project
	parts := strings.Split(strings.TrimPrefix(key, "otp_"), ".")
	if !strings.HasPrefix(key, "otp_") || len(parts) != 2 || !validID(parts[0]) || len(parts[1]) != 43 {
		return p, problem(401, "UNAUTHORIZED", "Invalid project credentials")
	}
	err := s.repo.db.WithContext(ctx).First(&p, "id = ?", parts[0]).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, problem(401, "UNAUTHORIZED", "Invalid project credentials")
	}
	if err != nil {
		return p, err
	}
	if !p.Enabled || !equal([]byte(p.KeyHash), []byte(s.digest("key", key))) {
		return p, problem(401, "UNAUTHORIZED", "Invalid project credentials")
	}
	return p, nil
}
func active(auth Project, current *Project) error {
	if !current.Enabled || !equal([]byte(auth.KeyHash), []byte(current.KeyHash)) {
		return problem(401, "UNAUTHORIZED", "Project credentials are no longer active")
	}
	return nil
}
func (s *Service) Rotate(ctx context.Context, id string) (string, error) {
	key := "otp_" + id + "." + randomSecret()
	err := s.repo.projectTx(ctx, id, func(tx *gorm.DB, p *Project) error {
		if err := tx.Model(p).Update("key_hash", s.digest("key", key)).Error; err != nil {
			return err
		}
		return audit(tx, id, "project.key_rotated", id)
	})
	return key, err
}
func (s *Service) Enable(ctx context.Context, id string, enabled bool) error {
	return s.repo.projectTx(ctx, id, func(tx *gorm.DB, p *Project) error {
		if err := tx.Model(p).Update("enabled", enabled).Error; err != nil {
			return err
		}
		if !enabled {
			if err := tx.Model(&Challenge{}).Where("project_id = ? AND state = ?", id, "ACTIVE").Updates(map[string]interface{}{"state": "CANCELLED", "digest": ""}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Job{}).Where("project_id = ? AND state IN ?", id, []string{"QUEUED", "SIMULATED"}).Updates(map[string]interface{}{"state": "CANCELLED", "payload": ""}).Error; err != nil {
				return err
			}
		}
		return audit(tx, id, "project.enabled_changed", id)
	})
}

type Request struct {
	Phone     string `json:"phone"`
	Purpose   string `json:"purpose"`
	Reference string `json:"reference"`
	Consent   bool   `json:"consent"`
}
type Verify struct {
	ChallengeID string `json:"challengeId"`
	Code        string `json:"code"`
	Purpose     string `json:"purpose"`
	Reference   string `json:"reference"`
}
type View struct {
	ChallengeID string    `json:"challengeId"`
	State       string    `json:"status"`
	Delivery    string    `json:"deliveryStatus"`
	Destination string    `json:"destination"`
	ExpiresAt   time.Time `json:"expiresAt"`
	ResendAfter time.Time `json:"resendAfter"`
	Sandbox     bool      `json:"sandbox"`
}

func (s *Service) view(tx *gorm.DB, c Challenge, p *Project) (View, error) {
	var j Job
	err := tx.First(&j, "challenge_id = ?", c.ID).Error
	state := c.State
	if state == "ACTIVE" && !s.now().Before(c.ExpiresAt) {
		state = "EXPIRED"
	}
	return View{c.ID, state, j.State, "••••" + c.Last4, c.ExpiresAt, c.CreatedAt.Add(time.Duration(p.Cooldown) * time.Second), j.Provider == "sandbox"}, err
}
func (s *Service) Request(ctx context.Context, p Project, in Request, key, ip string) (View, error) {
	var out View
	if !phonePattern.MatchString(in.Phone) || !purposePattern.MatchString(in.Purpose) || len(in.Reference) < 1 || len(in.Reference) > 128 || !in.Consent || !keyPattern.MatchString(key) {
		return out, problem(400, "INVALID_REQUEST", "Supply phone in E.164 format, purpose, reference, consent=true and an 8–128 character Idempotency-Key")
	}
	err := s.repo.projectTx(ctx, p.ID, func(tx *gorm.DB, current *Project) error {
		if err := active(p, current); err != nil {
			return err
		}
		var err error
		out, err = s.create(tx, current, in, key, ip, "")
		return err
	})
	return out, err
}
func (s *Service) create(tx *gorm.DB, p *Project, in Request, key, ip, resend string) (View, error) {
	var out View
	now := s.now().UTC()
	fp := s.digest("request", in.Phone, in.Purpose, in.Reference, resend)
	var prior Challenge
	err := tx.First(&prior, "project_id = ? AND idempotency_key = ?", p.ID, key).Error
	if err == nil {
		if prior.Fingerprint != fp {
			return out, problem(409, "IDEMPOTENCY_CONFLICT", "Key already used for another request")
		}
		return s.view(tx, prior, p)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	subject := s.digest("phone", in.Phone)
	var latest Challenge
	err = tx.Where("project_id = ? AND subject = ? AND purpose = ?", p.ID, subject, in.Purpose).Order("created_at DESC").First(&latest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	if resend != "" && (latest.ID != resend || latest.State == "VERIFIED" || latest.State == "SUPERSEDED" || latest.State == "CANCELLED") {
		return out, problem(409, "CHALLENGE_CLOSED", "Use the latest unverified challenge")
	}
	if latest.ID != "" && now.Before(latest.CreatedAt.Add(time.Duration(p.Cooldown)*time.Second)) {
		return out, &Error{Status: 429, Code: "COOLDOWN", Message: "Wait before requesting another code", RetryAfter: int(latest.CreatedAt.Add(time.Duration(p.Cooldown)*time.Second).Sub(now).Seconds()) + 1}
	}
	for _, lim := range []struct {
		k          string
		n, seconds int
	}{{"phone:" + subject, 10, 3600}, {"project:" + p.ID, p.DailyLimit, 86400}, {"ip:" + p.ID + ":" + s.digest("ip", ip), 60, 60}, {"subject:" + p.ID + ":" + subject, p.PhoneHourlyLimit, 3600}} {
		if err = rate(tx, lim.k, lim.n, lim.seconds, now); err != nil {
			return out, err
		}
	}
	if err = tx.Model(&Challenge{}).Where("project_id = ? AND subject = ? AND purpose = ? AND state = ?", p.ID, subject, in.Purpose, "ACTIVE").Updates(map[string]interface{}{"state": "SUPERSEDED", "digest": ""}).Error; err != nil {
		return out, err
	}
	sub := tx.Model(&Challenge{}).Select("id").Where("project_id = ? AND subject = ? AND purpose = ? AND state = ?", p.ID, subject, in.Purpose, "SUPERSEDED")
	if err = tx.Model(&Job{}).Where("challenge_id IN (?) AND state IN ?", sub, []string{"QUEUED", "SIMULATED"}).Updates(map[string]interface{}{"state": "CANCELLED", "payload": ""}).Error; err != nil {
		return out, err
	}
	id := uuid.NewString()
	code := newCode()
	phoneCipher, err := s.seal(in.Phone, "phone:"+id)
	if err != nil {
		return out, err
	}
	codeCipher, err := s.seal(code, "code:"+id)
	if err != nil {
		return out, err
	}
	c := Challenge{ID: id, ProjectID: p.ID, IdempotencyKey: key, Fingerprint: fp, Subject: subject, PhoneCipher: phoneCipher, Last4: in.Phone[len(in.Phone)-4:], Purpose: in.Purpose, Reference: in.Reference, Digest: s.digest("code", p.ID, id, code), State: "ACTIVE", MaxAttempts: p.MaxAttempts, ExpiresAt: now.Add(time.Duration(p.TTL) * time.Second), CreatedAt: now}
	if err = tx.Create(&c).Error; err != nil {
		return out, err
	}
	if err = tx.Create(&Job{ID: uuid.NewString(), ChallengeID: id, ProjectID: p.ID, Provider: p.Provider, Payload: codeCipher, State: "QUEUED", AvailableAt: now}).Error; err != nil {
		return out, err
	}
	if err = audit(tx, p.ID, "otp.requested_with_consent", id); err != nil {
		return out, err
	}
	return s.view(tx, c, p)
}
func (s *Service) Resend(ctx context.Context, p Project, id, key, ip string, consent bool) (View, error) {
	var out View
	if !validID(id) || !keyPattern.MatchString(key) || !consent {
		return out, problem(400, "INVALID_REQUEST", "Supply challengeId, consent=true and Idempotency-Key")
	}
	err := s.repo.projectTx(ctx, p.ID, func(tx *gorm.DB, current *Project) error {
		if err := active(p, current); err != nil {
			return err
		}
		var c Challenge
		if err := tx.First(&c, "id = ? AND project_id = ?", id, p.ID).Error; err != nil {
			return missing(err)
		}
		if c.PhoneCipher == "" {
			return problem(410, "EXPIRED", "Challenge retention has expired")
		}
		phone, err := s.open(c.PhoneCipher, "phone:"+id)
		if err != nil {
			return err
		}
		out, err = s.create(tx, current, Request{phone, c.Purpose, c.Reference, true}, key, ip, id)
		return err
	})
	return out, err
}
func missing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return problem(404, "NOT_FOUND", "Challenge not found")
	}
	return err
}

type Verification struct {
	Verified    bool   `json:"verified"`
	ChallengeID string `json:"challengeId"`
	Phone       string `json:"phone"`
	Purpose     string `json:"purpose"`
	Reference   string `json:"reference"`
	Sandbox     bool   `json:"sandbox"`
}

func (s *Service) Verify(ctx context.Context, p Project, in Verify, ip string) (Verification, error) {
	var out Verification
	if !validID(in.ChallengeID) || !codePattern.MatchString(in.Code) || !purposePattern.MatchString(in.Purpose) || len(in.Reference) < 1 || len(in.Reference) > 128 {
		return out, problem(400, "INVALID_REQUEST", "Invalid verification fields")
	}
	err := s.repo.projectTx(ctx, p.ID, func(tx *gorm.DB, current *Project) error {
		if err := active(p, current); err != nil {
			return err
		}
		now := s.now().UTC()
		if err := rate(tx, "verify:"+p.ID+":"+s.digest("ip", ip), 120, 60, now); err != nil {
			return err
		}
		var c Challenge
		if err := tx.First(&c, "id = ? AND project_id = ?", in.ChallengeID, p.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if c.State != "ACTIVE" || c.Digest == "" {
			return nil
		}
		if !now.Before(c.ExpiresAt) {
			return tx.Model(&c).Updates(map[string]interface{}{"state": "EXPIRED", "digest": ""}).Error
		}
		matches := equal([]byte(c.Digest), []byte(s.digest("code", p.ID, c.ID, in.Code))) && c.Purpose == in.Purpose && c.Reference == in.Reference
		if !matches {
			c.Attempts++
			if c.Attempts >= c.MaxAttempts {
				c.State = "LOCKED"
				c.Digest = ""
				if err := tx.Model(&Job{}).Where("challenge_id = ? AND state IN ?", c.ID, []string{"QUEUED", "SIMULATED"}).Updates(map[string]interface{}{"state": "CANCELLED", "payload": ""}).Error; err != nil {
					return err
				}
			}
			if err := tx.Save(&c).Error; err != nil {
				return err
			}
			return audit(tx, p.ID, "otp.verification_failed", c.ID)
		}
		var job Job
		if err := tx.First(&job, "challenge_id = ?", c.ID).Error; err != nil {
			return err
		}
		if !strings.Contains("|SENDING|ACCEPTED|DELIVERED|READ|UNKNOWN|SIMULATED|", "|"+job.State+"|") {
			return nil
		}
		phone, err := s.open(c.PhoneCipher, "phone:"+c.ID)
		if err != nil {
			return err
		}
		if err = tx.Model(&c).Updates(map[string]interface{}{"state": "VERIFIED", "digest": "", "verified_at": now}).Error; err != nil {
			return err
		}
		if err = tx.Model(&job).Update("payload", "").Error; err != nil {
			return err
		}
		if err = audit(tx, p.ID, "otp.verified", c.ID); err != nil {
			return err
		}
		out = Verification{true, c.ID, phone, c.Purpose, c.Reference, job.Provider == "sandbox"}
		return nil
	})
	if err == nil && !out.Verified {
		err = problem(400, "INVALID_OTP", "Code or challenge is invalid, expired, or already used")
	}
	return out, err
}
func (s *Service) Status(ctx context.Context, p Project, id string) (View, error) {
	var out View
	err := s.repo.projectTx(ctx, p.ID, func(tx *gorm.DB, current *Project) error {
		if err := active(p, current); err != nil {
			return err
		}
		var c Challenge
		if err := tx.First(&c, "id = ? AND project_id = ?", id, p.ID).Error; err != nil {
			return missing(err)
		}
		var err error
		out, err = s.view(tx, c, current)
		return err
	})
	return out, err
}
