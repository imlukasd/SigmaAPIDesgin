CREATE TABLE users (
	id uuid PRIMARY KEY,
	email text NOT NULL,
	email_normalized text NOT NULL,
	password_hash text NOT NULL,
	display_name text NOT NULL,
	status text NOT NULL DEFAULT 'active',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT users_email_not_blank CHECK (length(btrim(email)) > 0),
	CONSTRAINT users_email_normalized_not_blank CHECK (length(btrim(email_normalized)) > 0),
	CONSTRAINT users_email_normalized_format CHECK (email_normalized = lower(btrim(email_normalized))),
	CONSTRAINT users_password_hash_not_blank CHECK (length(btrim(password_hash)) > 0),
	CONSTRAINT users_display_name_not_blank CHECK (length(btrim(display_name)) > 0),
	CONSTRAINT users_status_valid CHECK (status IN ('active', 'disabled'))
);

ALTER TABLE users
	ADD CONSTRAINT users_email_normalized_unique UNIQUE (email_normalized);

CREATE TABLE organizations (
	id uuid PRIMARY KEY,
	name text NOT NULL,
	slug text NOT NULL,
	status text NOT NULL DEFAULT 'active',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT organizations_name_not_blank CHECK (length(btrim(name)) > 0),
	CONSTRAINT organizations_slug_not_blank CHECK (length(btrim(slug)) > 0),
	CONSTRAINT organizations_slug_format CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
	CONSTRAINT organizations_status_valid CHECK (status IN ('active', 'disabled'))
);

ALTER TABLE organizations
	ADD CONSTRAINT organizations_slug_unique UNIQUE (slug);

CREATE TABLE organization_memberships (
	organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role text NOT NULL,
	status text NOT NULL DEFAULT 'active',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (organization_id, user_id),
	CONSTRAINT organization_memberships_role_valid CHECK (role IN ('owner', 'admin', 'member')),
	CONSTRAINT organization_memberships_status_valid CHECK (status IN ('active', 'disabled'))
);

CREATE INDEX organization_memberships_user_id_idx
	ON organization_memberships (user_id);

CREATE INDEX organization_memberships_organization_id_role_idx
	ON organization_memberships (organization_id, role);
