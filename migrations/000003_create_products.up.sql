CREATE TABLE products (
	id uuid PRIMARY KEY,
	organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
	sku text NOT NULL,
	name text NOT NULL,
	description text,
	price_amount bigint NOT NULL,
	price_currency text NOT NULL,
	status text NOT NULL DEFAULT 'draft',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT products_sku_not_blank CHECK (length(btrim(sku)) > 0),
	CONSTRAINT products_sku_format CHECK (sku = upper(btrim(sku)) AND sku ~ '^[A-Z0-9][A-Z0-9_-]{0,63}$'),
	CONSTRAINT products_name_not_blank CHECK (length(btrim(name)) > 0),
	CONSTRAINT products_description_not_blank CHECK (description IS NULL OR length(btrim(description)) > 0),
	CONSTRAINT products_price_amount_non_negative CHECK (price_amount >= 0),
	CONSTRAINT products_price_currency_format CHECK (price_currency = upper(btrim(price_currency)) AND price_currency ~ '^[A-Z]{3}$'),
	CONSTRAINT products_status_valid CHECK (status IN ('draft', 'active', 'archived')),
	CONSTRAINT products_updated_after_created CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX products_organization_id_sku_unique
	ON products (organization_id, sku);

CREATE INDEX products_organization_id_created_at_id_idx
	ON products (organization_id, created_at DESC, id DESC);

CREATE INDEX products_organization_id_status_created_at_id_idx
	ON products (organization_id, status, created_at DESC, id DESC);
