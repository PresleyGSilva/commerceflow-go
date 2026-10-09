-- 005_stock_reserved: orders.stock_reserved was introduced after 004 had
-- already been applied on some databases, so it ships as its own migration
-- (never edit an applied migration). True while the order currently holds
-- decremented stock: checkout sets it on paid; decline/fail rollback, cancel
-- and terminal transitions clear it. Prevents double-restore for reopened
-- (failed -> pending) orders and drives acquire-on-pending->paid.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS stock_reserved BOOLEAN NOT NULL DEFAULT false;
-- Backfill for orders created between 004 and 005 that hold stock.
UPDATE orders SET stock_reserved = true WHERE status IN ('pending', 'paid') AND stock_reserved = false;
