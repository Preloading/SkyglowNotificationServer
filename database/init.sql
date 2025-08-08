CREATE TABLE IF NOT EXISTS queued_messages (
  message_id VARCHAR(36) NOT NULL,
  device_address VARCHAR(64) NOT NULL,
  routing_key BLOB NOT NULL,
  message VARCHAR(255) NOT NULL,
  topic VARCHAR(64) NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (`message_id`)
);

CREATE TABLE IF NOT EXISTS devices (
  device_address VARCHAR(64) NOT NULL,
  pub_key BLOB NOT NULL,
  lang VARCHAR(8) NOT NULL,
  PRIMARY KEY (`device_address`)
);

CREATE TABLE IF NOT EXISTS notification_tokens (
  routing_token BLOB NOT NULL,
  device_address VARCHAR(64) NOT NULL,
  allowed_notification_types integer NOT NULL,
  bundle_id VARCHAR(64) NOT NULL,
  issued_at DATETIME NOT NULL,
  is_valid integer NOT NULL,
  last_used DATETIME,
  PRIMARY KEY (`routing_token`)
);