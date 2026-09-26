-- 5 mock users for dev auth (signed-cookie session selects which one).
-- After migration 0009 the legacy is_pro column is gone; tier replaces it.

INSERT INTO users (handle, display_name, region, tier, avatar_color) VALUES
    ('john',  'John Smith',    'AU', 'pro',      '#7dd3fc'),
    ('alice', 'Alice Wang',    'AU', 'standard', '#c4b5fd'),
    ('bob',   'Bob Martinez',  'US', 'pro',      '#fcd34d'),
    ('carol', 'Carol Lee',     'SG', 'standard', '#86efac'),
    ('dan',   'Dan Patel',     'IN', 'standard', '#fca5a5')
ON CONFLICT (handle) DO NOTHING;
