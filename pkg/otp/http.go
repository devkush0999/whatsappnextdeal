package otp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var assets embed.FS

func writeError(c *gin.Context, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = problem(500, "INTERNAL_ERROR", "The request could not be completed")
	}
	if e.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(e.RetryAfter))
	}
	c.AbortWithStatusJSON(e.Status, gin.H{"error": e})
}
func decode(c *gin.Context, v interface{}) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(c, problem(400, "INVALID_JSON", "Invalid JSON body or unknown fields"))
		return false
	}
	var extra interface{}
	if dec.Decode(&extra) != io.EOF {
		writeError(c, problem(400, "INVALID_JSON", "Supply one JSON object"))
		return false
	}
	return true
}
func bearer(c *gin.Context) string {
	return strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
}
func (s *Service) security(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	c.Header("Access-Control-Allow-Origin", "")
	c.Header("Access-Control-Allow-Credentials", "")
	// Bound unauthenticated traffic using shared PostgreSQL buckets, without storing raw IPs.
	err := s.repo.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error { return rate(tx, "http:"+s.digest("ip", c.RemoteIP()), 300, 60, s.now()) })
	if err != nil {
		writeError(c, err)
		return
	}
	c.Next()
}
func (s *Service) admin(c *gin.Context) {
	if !equal([]byte(bearer(c)), []byte(s.cfg.AdminToken)) {
		writeError(c, problem(401, "UNAUTHORIZED", "Invalid admin token"))
		return
	}
	c.Next()
}
func (s *Service) project(c *gin.Context) {
	p, err := s.Authenticate(c.Request.Context(), bearer(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.Set("otpProject", p)
	c.Next()
}
func projectFrom(c *gin.Context) Project { return c.MustGet("otpProject").(Project) }
func idParam(c *gin.Context) bool {
	if !validID(c.Param("id")) {
		writeError(c, problem(400, "INVALID_ID", "Invalid identifier"))
		return false
	}
	return true
}

func (s *Service) Register(r *gin.Engine) {
	r.GET("/otp/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		var result int
		if err := s.repo.db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error; err != nil {
			c.JSON(503, gin.H{"status": "unavailable"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"status": "ok"})
	})
	ui := r.Group("/otp", s.security)
	ui.GET("", func(c *gin.Context) {
		b, _ := assets.ReadFile("web/index.html")
		c.Data(200, "text/html; charset=utf-8", b)
	})
	for _, file := range []string{"app.js", "style.css"} {
		file := file
		ui.GET("/"+file, func(c *gin.Context) {
			b, _ := assets.ReadFile("web/" + file)
			mime := "text/javascript; charset=utf-8"
			if strings.HasSuffix(file, "css") {
				mime = "text/css; charset=utf-8"
			}
			c.Data(200, mime, b)
		})
	}
	admin := r.Group("/otp/admin", s.security, s.admin)
	admin.GET("/overview", s.overview)
	admin.GET("/projects", func(c *gin.Context) {
		var p []Project
		if err := s.repo.db.WithContext(c.Request.Context()).Order("created_at DESC").Limit(200).Find(&p).Error; err != nil {
			writeError(c, err)
			return
		}
		c.JSON(200, gin.H{"projects": p})
	})
	admin.POST("/projects", func(c *gin.Context) {
		var in NewProject
		if !decode(c, &in) {
			return
		}
		p, key, err := s.CreateProject(c.Request.Context(), in)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(201, gin.H{"project": p, "apiKey": key})
	})
	admin.POST("/projects/:id/rotate", func(c *gin.Context) {
		if !idParam(c) {
			return
		}
		key, err := s.Rotate(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(200, gin.H{"apiKey": key})
	})
	admin.PATCH("/projects/:id", func(c *gin.Context) {
		if !idParam(c) {
			return
		}
		var in struct {
			Enabled *bool `json:"enabled"`
		}
		if !decode(c, &in) {
			return
		}
		if in.Enabled == nil {
			writeError(c, problem(400, "INVALID_REQUEST", "Supply enabled"))
			return
		}
		if err := s.Enable(c.Request.Context(), c.Param("id"), *in.Enabled); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(200, gin.H{"enabled": *in.Enabled})
	})
	admin.GET("/activity", s.activity)
	admin.POST("/sandbox/:id/code", s.sandboxCode)
	api := r.Group("/v1/otp", s.security, s.project)
	api.POST("/request", func(c *gin.Context) {
		var in Request
		if !decode(c, &in) {
			return
		}
		out, err := s.Request(c.Request.Context(), projectFrom(c), in, c.GetHeader("Idempotency-Key"), c.RemoteIP())
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(202, out)
	})
	api.POST("/resend", func(c *gin.Context) {
		var in struct {
			ChallengeID string `json:"challengeId"`
			Consent     bool   `json:"consent"`
		}
		if !decode(c, &in) {
			return
		}
		out, err := s.Resend(c.Request.Context(), projectFrom(c), in.ChallengeID, c.GetHeader("Idempotency-Key"), c.RemoteIP(), in.Consent)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(202, out)
	})
	api.POST("/verify", func(c *gin.Context) {
		var in Verify
		if !decode(c, &in) {
			return
		}
		out, err := s.Verify(c.Request.Context(), projectFrom(c), in, c.RemoteIP())
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(200, out)
	})
	api.GET("/challenges/:id", func(c *gin.Context) {
		if !idParam(c) {
			return
		}
		out, err := s.Status(c.Request.Context(), projectFrom(c), c.Param("id"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(200, out)
	})
	r.GET("/otp/webhooks/meta", s.metaVerify)
	r.POST("/otp/webhooks/meta", s.metaWebhook)
}
func (s *Service) overview(c *gin.Context) {
	var projects, active, verified, failed int64
	db := s.repo.db.WithContext(c.Request.Context())
	for _, query := range []struct {
		q *gorm.DB
		n *int64
	}{{db.Model(&Project{}), &projects}, {db.Model(&Challenge{}).Where("state = ? AND expires_at > ?", "ACTIVE", s.now()), &active}, {db.Model(&Challenge{}).Where("state = ? AND verified_at > ?", "VERIFIED", s.now().Add(-24*time.Hour)), &verified}, {db.Model(&Job{}).Where("state IN ?", []string{"FAILED", "UNKNOWN"}), &failed}} {
		if err := query.q.Count(query.n).Error; err != nil {
			writeError(c, err)
			return
		}
	}
	c.JSON(200, gin.H{"projects": projects, "active": active, "verifiedToday": verified, "needsAttention": failed, "providers": gin.H{"sandbox": s.cfg.AllowSandbox, "evolution": s.cfg.AllowEvolution, "meta": s.cfg.MetaToken != ""}})
}
func (s *Service) activity(c *gin.Context) {
	var jobs []Job
	db := s.repo.db.WithContext(c.Request.Context())
	query := db.Order("created_at DESC").Limit(100)
	if id := c.Query("projectId"); id != "" {
		if !validID(id) {
			writeError(c, problem(400, "INVALID_ID", "Invalid project identifier"))
			return
		}
		query = query.Where("project_id = ?", id)
	}
	if err := query.Find(&jobs).Error; err != nil {
		writeError(c, err)
		return
	}
	var audits []Audit
	if err := db.Order("created_at DESC").Limit(100).Find(&audits).Error; err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"jobs": jobs, "audits": audits})
}
func (s *Service) sandboxCode(c *gin.Context) {
	if !idParam(c) {
		return
	}
	if !s.cfg.AllowSandbox {
		writeError(c, problem(404, "NOT_FOUND", "Sandbox disabled"))
		return
	}
	var challenge Challenge
	var job Job
	db := s.repo.db.WithContext(c.Request.Context())
	if err := db.First(&challenge, "id = ?", c.Param("id")).Error; err != nil {
		writeError(c, missing(err))
		return
	}
	var code string
	err := s.repo.projectTx(c.Request.Context(), challenge.ProjectID, func(tx *gorm.DB, p *Project) error {
		if err := tx.First(&challenge, "id = ?", challenge.ID).Error; err != nil {
			return err
		}
		if err := tx.First(&job, "challenge_id = ?", challenge.ID).Error; err != nil {
			return err
		}
		if !p.Enabled || job.Provider != "sandbox" || job.State != "SIMULATED" || challenge.State != "ACTIVE" || !s.now().Before(challenge.ExpiresAt) {
			return problem(409, "NOT_AVAILABLE", "No active sandbox code is available")
		}
		var err error
		code, err = s.open(job.Payload, "code:"+challenge.ID)
		if err != nil {
			return err
		}
		return audit(tx, p.ID, "sandbox.code_viewed", challenge.ID)
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": code, "sandbox": true})
}
func (s *Service) metaVerify(c *gin.Context) {
	if s.cfg.MetaVerifyToken == "" || !equal([]byte(c.Query("hub.verify_token")), []byte(s.cfg.MetaVerifyToken)) || c.Query("hub.mode") != "subscribe" {
		c.Status(403)
		return
	}
	c.String(200, c.Query("hub.challenge"))
}
func (s *Service) metaWebhook(c *gin.Context) {
	if s.cfg.MetaAppSecret == "" {
		c.Status(404)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(413)
		return
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.MetaAppSecret))
	mac.Write(raw)
	sig, err := hex.DecodeString(strings.TrimPrefix(c.GetHeader("X-Hub-Signature-256"), "sha256="))
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		c.Status(401)
		return
	}
	var body struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						PhoneNumberID string `json:"phone_number_id"`
					} `json:"metadata"`
					Statuses []struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(raw, &body) != nil {
		c.Status(400)
		return
	}
	for _, entry := range body.Entry {
		for _, change := range entry.Changes {
			if change.Value.Metadata.PhoneNumberID != s.cfg.MetaPhoneID {
				continue
			}
			for _, status := range change.Value.Statuses {
				if err := s.RecordReceipt(c.Request.Context(), status.ID, status.Status); err != nil {
					c.Status(503)
					return
				}

			}
		}
	}
	if err := s.ReconcileReceipts(c.Request.Context()); err != nil {
		c.Status(503)
		return
	}
	c.Status(200)
}
