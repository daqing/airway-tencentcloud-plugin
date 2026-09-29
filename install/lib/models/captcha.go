package models

import (
	"time"
)

// Captcha is one issued captcha, bound to the IP that requested it.
type Captcha struct {
	ID        string    `db:"id" json:"id"`
	Answer    string    `db:"answer" json:"answer"`
	IP        string    `db:"ip" json:"ip"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	Consumed  bool      `db:"consumed" json:"consumed"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (Captcha) TableName() string {
	return "captchas"
}

// Expired reports whether the captcha's validity window has passed.
func (c Captcha) Expired() bool {
	return time.Now().After(c.ExpiresAt)
}

func init() {
	registerREPLModel("SmsCaptcha", Captcha{})
}
