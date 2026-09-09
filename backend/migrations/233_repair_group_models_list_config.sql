-- Repair databases where the historical group model-list migration was marked
-- applied but the column was lost during a restore.
ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb;
