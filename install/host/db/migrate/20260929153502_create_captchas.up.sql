CREATE TABLE captchas (
  id VARCHAR(32) PRIMARY KEY,
  answer VARCHAR(8) NOT NULL,
  ip VARCHAR(45) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX index_captchas_on_ip_created_at ON captchas (ip, created_at);
