-- Runs once, when the database volume is first created. The integration test truncates tables,
-- so it gets its own database rather than sharing the one the running service uses.
CREATE DATABASE compliance_test OWNER compliance;
