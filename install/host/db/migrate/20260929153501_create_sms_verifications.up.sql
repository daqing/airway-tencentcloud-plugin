CREATE TABLE sms_verifications (
  id BIGSERIAL PRIMARY KEY,
  phone VARCHAR(20) NOT NULL,
  code VARCHAR(6) NOT NULL,
  ip VARCHAR(45) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed BOOLEAN NOT NULL DEFAULT FALSE,
  attempts INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX index_sms_verifications_on_phone_created_at ON sms_verifications (phone, created_at);
CREATE INDEX index_sms_verifications_on_ip_created_at ON sms_verifications (ip, created_at);
