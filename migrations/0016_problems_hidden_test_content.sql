-- hidden_test_content used to be added lazily by the problem seeder at
-- boot (internal/problems/seed.go). Schema belongs in migrations.
ALTER TABLE problems ADD COLUMN IF NOT EXISTS hidden_test_content TEXT;
