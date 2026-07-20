-- TitipDong — cleanup all dummy test data from production DB.
--
-- Run via:
--   PGPASSWORD='<your-pw>' psql -h <truenas-ip> -U postgres -d titipdong \
--     -f scripts/cleanup-dummy-data.sql
--
-- Safe behaviors:
--   - KEEP the admin account you actually use (rifianggriawan@gmail.com).
--   - Delete all dummy users (buyer2/jastiper2/t1/kyctest and any *@x.com / *@dummy.test).
--   - Cascade-delete all related rows (orders, customers, trips, KYC apps, catalog).
--   - Reset SERIAL counters so new rows start from id=2 (admin stays at id=1).
--   - DRY RUN: comment out the actual DELETE lines below to preview first.
--
-- IMPORTANT: review the SELECT output below before running deletes.

BEGIN;

-- Preview what will be kept / deleted.
\echo '=== Users that will be KEPT (admin) ==='
SELECT id, email, role FROM users
 WHERE email = 'rifianggriawan@gmail.com';

\echo '=== Users that will be DELETED (dummy) ==='
SELECT id, email, role FROM users
 WHERE email != 'rifianggriawan@gmail.com'
 ORDER BY id;

\echo '=== Rows that will be cascade-deleted ==='
SELECT 'orders' AS table_name, count(*) FROM orders
UNION ALL SELECT 'customers', count(*) FROM customers
UNION ALL SELECT 'trips', count(*) FROM trips
UNION ALL SELECT 'catalog_items', count(*) FROM catalog_items
UNION ALL SELECT 'jastiper_applications', count(*) FROM jastiper_applications;

-- === ACTUAL CLEANUP ===
-- Deletes all users except your admin; FK ON DELETE CASCADE removes their data.
DELETE FROM users WHERE email != 'rifianggriawan@gmail.com';

-- Belt-and-suspenders: clear any orphaned rows that survived cascade.
TRUNCATE orders, customers, trips, catalog_items, jastiper_applications, fx_rates
       RESTART IDENTITY CASCADE;

-- Reset the users sequence so the next signup gets id=2 (admin keeps id=1).
SELECT setval('users_id_seq', (SELECT max(id) FROM users));

COMMIT;

\echo '=== Final state ==='
SELECT id, email, role FROM users ORDER BY id;
SELECT 'orders:' || count(*) FROM orders
UNION ALL SELECT 'customers:' || count(*) FROM customers
UNION ALL SELECT 'trips:' || count(*) FROM trips
UNION ALL SELECT 'catalog:' || count(*) FROM catalog_items
UNION ALL SELECT 'kyc_apps:' || count(*) FROM jastiper_applications;
