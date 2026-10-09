-- +goose Up
-- +goose StatementBegin
-- A refresh token is no longer bound to the address it was issued to. The
-- binding compared against whichever hop connected last -- a proxy container,
-- the same for every user -- so it protected nothing, and bound to the real
-- client it would sign people out for changing networks. What the row keeps
-- instead is a record: the address of the request that minted it, written
-- fresh on every rotation, so the newest row of a family says where the
-- session was last used from.
--
-- Existing rows keep working: they get an empty value here and a real one on
-- their next rotation.
ALTER TABLE refresh_tokens
    DROP COLUMN bound_ip,
    ADD COLUMN client_ip TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Restores the shape, not the values: every row comes back with an empty
-- bound_ip, which the previous binary treats as a mismatch on the next
-- refresh -- rolling back signs everyone out once.
ALTER TABLE refresh_tokens
    DROP COLUMN client_ip,
    ADD COLUMN bound_ip TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
