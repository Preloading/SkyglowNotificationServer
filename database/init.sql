CREATE TABLE IF NOT EXISTS queued_messages (  
  created_at TIMESTAMP NOT NULL,

  is_encrypted BOOLEAN NOT NULL,

  -- unencrypted msg data
  alert_body VARCHAR(255),
  alert_action VARCHAR(255),
  alert_sound VARCHAR(255),
  badge_number integer,

  -- encrypted message data
  ciphertext BYTEA,
  data_type VARCHAR(8),
  iv BYTEA,
  
  -- routing info
  device_address VARCHAR(64) NOT NULL,
  routing_key BYTEA NOT NULL,
  message_id VARCHAR(36) NOT NULL,
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
  feedback_provider VARCHAR(16),
  allowed_notification_types integer NOT NULL,
  bundle_id VARCHAR(64) NOT NULL,
  issued_at TIMESTAMP NOT NULL,
  is_valid BOOLEAN NOT NULL,
  last_used TIMESTAMP,
  marked_for_removal_at TIMESTAMP,
  PRIMARY KEY ("routing_token")
);

-- other server's (& ours) feedback relation to the token
CREATE TABLE IF NOT EXISTS feedback_token (
  feedback_key BYTEA NOT NULL,
  routing_token BYTEA NOT NULL,
  routing_domain VARCHAR(16) NOT NULL,
  last_used TIMESTAMP NOT NULL,
  PRIMARY KEY ("routing_token")
);

-- feedback on service's trusted server
CREATE TABLE IF NOT EXISTS feedback_to_send (
  feedback_key BYTEA NOT NULL,
  routing_token BYTEA NOT NULL,
  server_address VARCHAR(16) NOT NULL,
  type integer NOT NULL, -- 1 = token deleted
  reason VARCHAR(64),
  created_at TIMESTAMP NOT NULL
);