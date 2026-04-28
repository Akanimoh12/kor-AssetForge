-- Seed Users
INSERT INTO users (id, email, password_hash, role, is_verified, created_at, updated_at)
VALUES 
    ('00000000-0000-0000-0000-000000000001', 'admin@assetforge.com', '$2a$12$LQv3c1yqBWVHxkd0LqG8xeL.QG2mXq8mXq8mXq8mXq8mXq8mXq8m.', 'admin', true, NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000002', 'user@assetforge.com', '$2a$12$LQv3c1yqBWVHxkd0LqG8xeL.QG2mXq8mXq8mXq8mXq8mXq8mXq8m.', 'user', true, NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

-- Seed User Balances
INSERT INTO user_balances (user_id, asset_code, balance, updated_at)
VALUES 
    ('00000000-0000-0000-0000-000000000001', 'XLM', 1000.0, NOW()),
    ('00000000-0000-0000-0000-000000000002', 'XLM', 500.0, NOW())
ON CONFLICT (user_id, asset_code) DO NOTHING;
