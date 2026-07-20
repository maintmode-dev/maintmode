-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- Composite index for the license heartbeat's last_activity_at query
--: MAX(created_at) WHERE action IN (domain whitelist), once per
-- minute. With only the separate (created_at DESC) and (action) indexes, a
-- login-heavy instance with no recent domain activity walks the created_at
-- index across every auth row before answering — exactly the dormant-customer
-- case Console wants to detect. The composite turns it into an index-only
-- scan over just the matching action groups.
CREATE INDEX idx_audit_log_action_created_at ON audit_log (action, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP INDEX idx_audit_log_action_created_at;
-- +goose StatementEnd
