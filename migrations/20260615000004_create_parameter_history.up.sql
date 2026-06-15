DO $$ BEGIN
    CREATE TYPE parameter_history_change_type_enum AS ENUM ('create', 'update', 'delete', 'activate', 'deactivate');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS parameter_history (
    id                               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parameter_id                     UUID NOT NULL,
    org_id                           UUID NOT NULL,
    previous_value                   JSONB,
    new_value                        JSONB NOT NULL DEFAULT '{}',
    change_type                      parameter_history_change_type_enum NOT NULL DEFAULT 'create',
    previous_version                 INTEGER NOT NULL DEFAULT 0,
    new_version                      INTEGER NOT NULL DEFAULT 0,
    change_reason                    VARCHAR(500) NOT NULL DEFAULT '',
    change_metadata                  JSONB NOT NULL DEFAULT '{}',
    changed_by                       UUID,
    changed_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_parameter_history_parameter_id FOREIGN KEY (parameter_id) REFERENCES parameters(id)
);

CREATE INDEX IF NOT EXISTS idx_parameter_history_parameter_id ON parameter_history(parameter_id);
CREATE INDEX IF NOT EXISTS idx_parameter_history_org_id ON parameter_history(org_id);
CREATE INDEX IF NOT EXISTS idx_param_history_changed_at
    ON parameter_history(org_id, changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_param_history_change_type
    ON parameter_history(org_id, change_type);