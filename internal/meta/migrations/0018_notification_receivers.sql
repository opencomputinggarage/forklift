-- Notification receivers: named alarm channels (e.g. a Slack/Mattermost webhook)
-- managed in the admin console. An enabled receiver gets a JSON POST when a
-- package is quarantined pending approval. Name is unique and used as the
-- human-facing channel identifier; description documents its purpose.

CREATE TABLE notification_receivers (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    webhook_url TEXT NOT NULL DEFAULT '',  -- write-only: never returned by the API after creation
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
