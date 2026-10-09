-- 002_seed: fictitious demo data. Money in CENTS.
INSERT INTO orgs(id, name) VALUES ('11111111-1111-1111-1111-111111111111', 'Loja Demo')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- admin@commerceflow.local / Admin123!
INSERT INTO users(id, email, password_hash, name, role) VALUES
('a0000000-0000-0000-0000-000000000001', 'admin@commerceflow.local', '$2a$12$j6ourqJCTISRzx/LADDyye87K8iXsOri0tjMIai40Z2YrOPGDsizW', 'Admin', 'admin')
ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, name = EXCLUDED.name, role = EXCLUDED.role;

-- ana@exemplo.com / Customer123!
INSERT INTO users(id, email, password_hash, name, role) VALUES
('a0000000-0000-0000-0000-000000000002', 'ana@exemplo.com', '$2a$12$KfHvcN0eIfMwWa/Ll7eWvuolVUxJw56yXBc7pMumZgUV1HXJGqs7S', 'Ana', 'customer')
ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, name = EXCLUDED.name, role = EXCLUDED.role;

INSERT INTO org_members(org_id, user_id, role) VALUES
('11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000001', 'owner'),
('11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000002', 'member')
ON CONFLICT DO NOTHING;

INSERT INTO categories(name, slug) VALUES
('Eletrônicos', 'eletronicos'),
('Moda', 'moda'),
('Casa', 'casa'),
('Esporte', 'esporte')
ON CONFLICT (slug) DO NOTHING;

-- ~12 products, prices in CENTS.
WITH c AS (SELECT slug, id FROM categories)
INSERT INTO products(name, slug, description, price_cents, stock_qty, category_id, image_url, active)
SELECT v.name, v.slug, v.description, v.price_cents, v.stock_qty, c.id, '', true
FROM c JOIN (VALUES
  ('Fone Bluetooth X1', 'fone-bluetooth-x1', 'Fone sem fio com cancelamento de ruído', 29990, 50, 'eletronicos'),
  ('Smartwatch Fit 2', 'smartwatch-fit-2', 'Relógio inteligente com GPS', 49990, 30, 'eletronicos'),
  ('Teclado Mecânico Pro', 'teclado-mecanico-pro', 'Teclado mecânico ABNT2 RGB', 34990, 25, 'eletronicos'),
  ('Camiseta Essencial', 'camiseta-essencial', 'Camiseta 100% algodão', 7990, 100, 'moda'),
  ('Jaqueta Jeans Classic', 'jaqueta-jeans-classic', 'Jaqueta jeans unissex', 24990, 40, 'moda'),
  ('Tênis Runner Air', 'tenis-runner-air', 'Tênis de corrida amortecido', 39990, 35, 'moda'),
  ('Luminária Mesa Nord', 'luminaria-mesa-nord', 'Luminária de mesa minimalista', 14990, 20, 'casa'),
  ('Jogo Panelas 5pçs', 'jogo-panelas-5pcs', 'Panelas antiaderentes 5 peças', 32990, 15, 'casa'),
  ('Cafeteira Espresso 19bar', 'cafeteira-espresso-19bar', 'Cafeteira espresso doméstica', 59990, 12, 'casa'),
  ('Bola Futebol Oficial', 'bola-futebol-oficial', 'Bola oficial costurada', 19990, 60, 'esporte'),
  ('Halteres Par 10kg', 'halteres-par-10kg', 'Par de halteres emborrachados', 27990, 18, 'esporte'),
  ('Mochila Trail 30L', 'mochila-trail-30l', 'Mochila impermeável 30 litros', 22990, 45, 'esporte')
) AS v(name, slug, description, price_cents, stock_qty, catslug)
ON v.catslug = c.slug
ON CONFLICT (slug) DO UPDATE SET price_cents = EXCLUDED.price_cents, stock_qty = EXCLUDED.stock_qty;

INSERT INTO coupons(code, type, value, min_subtotal_cents, active, expires_at) VALUES
('BEMVINDO10', 'percent', 10, 0, true, now() + interval '1 year'),
('FRETE5', 'fixed', 500, 0, true, now() + interval '1 year')
ON CONFLICT (code) DO UPDATE SET type = EXCLUDED.type, value = EXCLUDED.value, active = true;
