CREATE TABLE IF NOT EXISTS queued_messages (
  message_id VARCHAR(36) NOT NULL,
  device_address VARCHAR(64) NOT NULL,
  routing_key BYTEA NOT NULL,
  message VARCHAR(255) NOT NULL,
  topic VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL,
  PRIMARY KEY ("message_id")
);

CREATE TABLE IF NOT EXISTS devices (
  device_address VARCHAR(64) NOT NULL,
  pub_key BYTEA NOT NULL,
  lang VARCHAR(8) NOT NULL,
  PRIMARY KEY ("device_address")
);

CREATE TABLE IF NOT EXISTS notification_tokens (
  routing_token BYTEA NOT NULL,
  device_address VARCHAR(64) NOT NULL,
  allowed_notification_types integer NOT NULL,
  bundle_id VARCHAR(64) NOT NULL,
  issued_at TIMESTAMP NOT NULL,
  is_valid BOOLEAN NOT NULL,
  last_used TIMESTAMP,
  PRIMARY KEY ("routing_token")
);