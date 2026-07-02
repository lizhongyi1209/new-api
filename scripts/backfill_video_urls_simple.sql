-- Backfill video URLs from tasks.private_data.result_url to logs.other.video_url
-- This script only updates logs that already have a task_id field

BEGIN;

-- Update logs for Kling tasks (platform = '50')
WITH task_urls AS (
    SELECT
        task_id,
        private_data->>'result_url' as result_url
    FROM tasks
    WHERE platform = '50'
      AND status = 'SUCCESS'
      AND private_data->>'result_url' IS NOT NULL
      AND private_data->>'result_url' != ''
)
UPDATE logs
SET other = jsonb_set(
    other::jsonb,
    '{video_url}',
    to_jsonb(task_urls.result_url),
    true
)::text
FROM task_urls
WHERE logs.other::jsonb->>'task_id' = task_urls.task_id
  AND logs.type = 2
  AND logs.other::jsonb ? 'is_task'
  AND (logs.other::jsonb->>'video_url' IS NULL OR logs.other::jsonb->>'video_url' = '');

-- Update logs for Seedance/ServiceInference tasks (platform = '60')
WITH task_urls AS (
    SELECT
        task_id,
        private_data->>'result_url' as result_url
    FROM tasks
    WHERE platform = '60'
      AND status = 'SUCCESS'
      AND private_data->>'result_url' IS NOT NULL
      AND private_data->>'result_url' != ''
)
UPDATE logs
SET other = jsonb_set(
    other::jsonb,
    '{video_url}',
    to_jsonb(task_urls.result_url),
    true
)::text
FROM task_urls
WHERE logs.other::jsonb->>'task_id' = task_urls.task_id
  AND logs.type = 2
  AND logs.other::jsonb ? 'is_task'
  AND (logs.other::jsonb->>'video_url' IS NULL OR logs.other::jsonb->>'video_url' = '');

-- Update logs for Doubao Video tasks (platform = '54')
WITH task_urls AS (
    SELECT
        task_id,
        private_data->>'result_url' as result_url
    FROM tasks
    WHERE platform = '54'
      AND status = 'SUCCESS'
      AND private_data->>'result_url' IS NOT NULL
      AND private_data->>'result_url' != ''
)
UPDATE logs
SET other = jsonb_set(
    other::jsonb,
    '{video_url}',
    to_jsonb(task_urls.result_url),
    true
)::text
FROM task_urls
WHERE logs.other::jsonb->>'task_id' = task_urls.task_id
  AND logs.type = 2
  AND logs.other::jsonb ? 'is_task'
  AND (logs.other::jsonb->>'video_url' IS NULL OR logs.other::jsonb->>'video_url' = '');

COMMIT;

-- Show results
SELECT 'Backfill completed. Updated logs with video URLs.' as result;
