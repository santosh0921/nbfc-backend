-- Least-privilege application database role (Production Infrastructure
-- Readiness, Part 4).
--
-- FINDING THIS ADDRESSES: the application currently connects as
-- `postgres`, a full superuser (rolsuper, rolcreaterole, rolcreatedb,
-- rolbypassrls all true). Verified by query, not assumed. That means the
-- application's own credentials can DROP the append-only trigger on
-- audit_logs — the control that every prior phase relied on to make the
-- audit trail authoritative. A SQL-injection defect or a leaked
-- connection string would carry full DDL power over the database.
--
-- TWO ROLES, NOT ONE. The application runs AutoMigrate at startup, which
-- requires DDL. Granting the runtime user DDL would defeat the purpose,
-- so the correct split is:
--
--   nbfc_migrate  — DDL. Used ONLY by the deployment/migration step.
--   nbfc_app      — DML only. Used by the running application.
--
-- This requires a deployment change: migrations must run as a separate
-- step with the migration credential, and the application must start
-- with migrations already applied. See the readiness report.
--
-- USAGE (psql, as a superuser):
--   \set app_password 'value-from-your-secret-store'
--   \set migrate_password 'value-from-your-secret-store'
--   \i scripts/create_least_privilege_role.sql
--
-- Passwords are NEVER written in this file. They are supplied as psql
-- variables from the operator's secret store at run time.

\if :{?app_password}
\else
\echo 'ERROR: :app_password is not set. Supply it from your secret store.'
\quit
\endif

\if :{?migrate_password}
\else
\echo 'ERROR: :migrate_password is not set. Supply it from your secret store.'
\quit
\endif

-- === Runtime role: DML only, no DDL, no superuser ===
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nbfc_app') THEN
        CREATE ROLE nbfc_app LOGIN;
    END IF;
END $$;

ALTER ROLE nbfc_app WITH PASSWORD :'app_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS NOINHERIT;

-- === Migration role: DDL, still not a superuser ===
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nbfc_migrate') THEN
        CREATE ROLE nbfc_migrate LOGIN;
    END IF;
END $$;

ALTER ROLE nbfc_migrate WITH PASSWORD :'migrate_password'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;

-- === Grants ===

-- Revoke the implicit PUBLIC create right on the schema so neither role
-- can add objects it should not.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

GRANT CONNECT ON DATABASE :"DBNAME" TO nbfc_app, nbfc_migrate;
GRANT USAGE ON SCHEMA public TO nbfc_app, nbfc_migrate;

-- Runtime: read and write rows. NO DDL, so it cannot ALTER TABLE, and
-- cannot DROP the audit_logs append-only trigger.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO nbfc_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO nbfc_app;

-- Migration role owns schema change.
GRANT CREATE ON SCHEMA public TO nbfc_migrate;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO nbfc_migrate;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO nbfc_migrate;

-- Objects created later by the migration role must still be usable by
-- the runtime role.
ALTER DEFAULT PRIVILEGES FOR ROLE nbfc_migrate IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO nbfc_app;
ALTER DEFAULT PRIVILEGES FOR ROLE nbfc_migrate IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO nbfc_app;

-- NOTE ON audit_logs: nbfc_app deliberately KEEPS INSERT (the
-- application must be able to write audit rows) but has no DDL, so the
-- append-only trigger that rejects UPDATE/DELETE cannot be removed by
-- the application's own credentials. The trigger continues to reject
-- UPDATE/DELETE for every role that is not a superuser or the table
-- owner.

\echo 'Roles nbfc_app (DML only) and nbfc_migrate (DDL) configured.'
\echo 'Set the application DB_USER to nbfc_app and run migrations separately as nbfc_migrate.'
