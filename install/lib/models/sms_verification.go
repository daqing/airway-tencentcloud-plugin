package models

import (
	"time"

	"github.com/daqing/airway/lib/sql"
)

// SmsVerification is one delivered verification code. Codes are stored in
// plaintext: they are short-lived, single-use, and rate-limited, so a hash
// would buy nothing an attacker who can already read this table lacks.
type SmsVerification struct {
	ID        sql.IdType `db:"id" json:"id"`
	Phone     string     `db:"phone" json:"phone"`
	Code      string     `db:"code" json:"code"`
	IP        string     `db:"ip" json:"ip"`
	ExpiresAt time.Time  `db:"expires_at" json:"expires_at"`
	Consumed  bool       `db:"consumed" json:"consumed"`
	Attempts  int        `db:"attempts" json:"attempts"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

func (SmsVerification) TableName() string {
	return "sms_verifications"
}

// Expired reports whether the code's validity window has passed.
func (v SmsVerification) Expired() bool {
	return time.Now().After(v.ExpiresAt)
}

func init() {
	registerREPLModel("SmsVerification", SmsVerification{})
}
