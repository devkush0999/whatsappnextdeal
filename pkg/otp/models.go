package otp

import "time"

type Project struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey"`
	Name             string    `json:"name" gorm:"size:80;not null"`
	KeyHash          string    `json:"-" gorm:"not null"`
	Enabled          bool      `json:"enabled"`
	Provider         string    `json:"provider" gorm:"size:16;not null"`
	InstanceID       string    `json:"instanceId,omitempty" gorm:"size:36"`
	Template         string    `json:"template,omitempty" gorm:"size:128"`
	Language         string    `json:"language" gorm:"size:16"`
	TTL              int       `json:"ttlSeconds"`
	MaxAttempts      int       `json:"maxAttempts"`
	Cooldown         int       `json:"cooldownSeconds"`
	PhoneHourlyLimit int       `json:"phoneHourlyLimit"`
	DailyLimit       int       `json:"dailyLimit"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (Project) TableName() string { return "otp_projects" }

type Challenge struct {
	ID             string     `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID      string     `json:"projectId" gorm:"type:uuid;not null;uniqueIndex:otp_idempotency;index:otp_subject,priority:1;index:otp_project_created,priority:1"`
	IdempotencyKey string     `json:"-" gorm:"size:128;not null;uniqueIndex:otp_idempotency"`
	Fingerprint    string     `json:"-"`
	Subject        string     `json:"-" gorm:"index:otp_subject,priority:2"`
	PhoneCipher    string     `json:"-"`
	Last4          string     `json:"destination" gorm:"size:4"`
	Purpose        string     `json:"purpose" gorm:"size:40;index:otp_subject,priority:3"`
	Reference      string     `json:"reference" gorm:"size:128"`
	Digest         string     `json:"-"`
	State          string     `json:"status" gorm:"size:16;index"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"maxAttempts"`
	ExpiresAt      time.Time  `json:"expiresAt" gorm:"index"`
	CreatedAt      time.Time  `json:"createdAt" gorm:"index:otp_subject,priority:4;index:otp_project_created,priority:2"`
	VerifiedAt     *time.Time `json:"verifiedAt,omitempty"`
}

func (Challenge) TableName() string { return "otp_challenges" }

type Job struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey"`
	ChallengeID string     `json:"challengeId" gorm:"type:uuid;uniqueIndex;not null"`
	ProjectID   string     `json:"projectId" gorm:"type:uuid;index"`
	Provider    string     `json:"provider" gorm:"size:16"`
	Payload     string     `json:"-"`
	State       string     `json:"status" gorm:"size:16;index:otp_queue,priority:1"`
	Attempts    int        `json:"attempts"`
	AvailableAt time.Time  `json:"availableAt" gorm:"index:otp_queue,priority:2"`
	StartedAt   *time.Time `json:"startedAt,omitempty" gorm:"index"`
	ProviderID  string     `json:"providerId,omitempty" gorm:"index"`
	ErrorCode   string     `json:"errorCode,omitempty" gorm:"size:64"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (Job) TableName() string { return "otp_jobs" }

type Rate struct {
	Key       string `gorm:"primaryKey;size:160"`
	Count     int
	ExpiresAt time.Time `gorm:"index"`
}

func (Rate) TableName() string { return "otp_rates" }

type Audit struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID  string    `json:"projectId" gorm:"type:uuid;index"`
	Action     string    `json:"action" gorm:"size:60"`
	ResourceID string    `json:"resourceId,omitempty" gorm:"size:36"`
	CreatedAt  time.Time `json:"createdAt" gorm:"index"`
}

func (Audit) TableName() string { return "otp_audits" }

// Receipt retains authenticated Meta status events that can precede the send response.
type Receipt struct {
	ProviderID string `gorm:"primaryKey;size:256"`
	State      string `gorm:"size:16"`
	Rank       int
	CreatedAt  time.Time `gorm:"index"`
}

func (Receipt) TableName() string { return "otp_receipts" }
