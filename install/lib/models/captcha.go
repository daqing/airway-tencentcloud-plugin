package models

import (
	"time"

	"github.com/daqing/airway/lib/sql"
)

// Captcha is one issued captcha, bound to the IP that requested it. Token is
// the identifier handed to the client; ID is the row's own key and never
// leaves the server.
type Captcha struct {
	ID        sql.IdType `db:"id" json:"id"`
	Token     string     `db:"token" json:"token"`
	Answer    string     `db:"answer" json:"answer"`
	IP        string     `db:"ip" json:"ip"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	Consumed  bool       `db:"consumed" json:"consumed"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
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
