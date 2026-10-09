-- 003_catalog: additive columns/indexes on top of 001_core (non-prefixed tables).
-- 001 already creates categories, products, coupons, addresses, stock_movements.

-- Categories: description + updated_at for admin CRUD.
ALTER TABLE categories ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE categories ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Products: search/sort indexes (columns already exist in 001).
CREATE INDEX IF NOT EXISTS idx_products_slug ON products(slug);
CREATE INDEX IF NOT EXISTS idx_products_price ON products(price_cents);
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
CREATE INDEX IF NOT EXISTS idx_products_active ON products(active);

-- Coupons: updated_at + code index.
ALTER TABLE coupons ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS idx_coupons_code ON coupons(code);

-- Addresses: extra fields for checkout/shipping.
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT '';
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS zip TEXT NOT NULL DEFAULT '';
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS country TEXT NOT NULL DEFAULT 'BR';
ALTER TABLE addresses ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS idx_addresses_user ON addresses(user_id);

-- Stock movements (001 shape: product_id, order_id NULLABLE, qty SIGNED, reason):
-- manual admin adjustments use qty = delta, order_id = NULL;
-- checkout (orders slice) uses qty = -item_qty, order_id = order id.
