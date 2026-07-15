CREATE TABLE auth_sessions (
	id uuid PRIMARY KEY,
	user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	user_agent text,
	ip_address inet,
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz,
	last_used_at timestamptz,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT auth_sessions_expires_after_created CHECK (expires_at > created_at),
	CONSTRAINT auth_sessions_revoked_after_created CHECK (revoked_at IS NULL OR revoked_at >= created_at),
	CONSTRAINT auth_sessions_last_used_after_created CHECK (last_used_at IS NULL OR last_used_at >= created_at),
	CONSTRAINT auth_sessions_user_agent_not_blank CHECK (user_agent IS NULL OR length(btrim(user_agent)) > 0)
);

CREATE INDEX auth_sessions_user_id_active_idx
	ON auth_sessions (user_id, expires_at)
	WHERE revoked_at IS NULL;

CREATE INDEX auth_sessions_expires_at_idx
	ON auth_sessions (expires_at);

CREATE TABLE refresh_tokens (
	id uuid PRIMARY KEY,
	session_id uuid NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
	token_hash text NOT NULL,
	rotated_from_token_id uuid REFERENCES refresh_tokens(id) ON DELETE SET NULL,
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz,
	last_used_at timestamptz,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT refresh_tokens_token_hash_not_blank CHECK (length(btrim(token_hash)) > 0),
	CONSTRAINT refresh_tokens_expires_after_created CHECK (expires_at > created_at),
	CONSTRAINT refresh_tokens_revoked_after_created CHECK (revoked_at IS NULL OR revoked_at >= created_at),
	CONSTRAINT refresh_tokens_last_used_after_created CHECK (last_used_at IS NULL OR last_used_at >= created_at),
	CONSTRAINT refresh_tokens_not_rotated_from_self CHECK (rotated_from_token_id IS NULL OR rotated_from_token_id <> id)
);

ALTER TABLE refresh_tokens
	ADD CONSTRAINT refresh_tokens_token_hash_unique UNIQUE (token_hash);

CREATE UNIQUE INDEX refresh_tokens_rotated_from_token_id_unique
	ON refresh_tokens (rotated_from_token_id)
	WHERE rotated_from_token_id IS NOT NULL;

CREATE INDEX refresh_tokens_session_id_active_idx
	ON refresh_tokens (session_id, expires_at)
	WHERE revoked_at IS NULL;

CREATE INDEX refresh_tokens_expires_at_idx
	ON refresh_tokens (expires_at);
