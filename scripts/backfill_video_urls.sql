-- Backfill video URLs from tasks.private_data.result_url to logs.other.video_url
-- This script updates historical task logs to include video URLs

BEGIN;

-- Update logs for Kling tasks (platform = '50')
WITH task_urls AS (
    SELECT
        task_id,
        COALESCE(
            private_data->>'result_url',
            CASE
                WHEN data::text != '{}' AND data::text != '' THEN
                    COALESCE(
                        data::jsonb->'data'->'task_result'->'videos'->0->>'url',
                        data::jsonb->>'video_url',
                        data::jsonb->>'url'
                    )
            END
        ) as result_url
    FROM tasks
    WHERE platform = '50'
      AND status = 'SUCCESS'
      AND COALESCE(
          private_data->>'result_url',
          data::jsonb->'data'->'task_result'->'videos'->0->>'url',
          data::jsonb->>'video_url',
          data::jsonb->>'url'
      ) IS NOT NULL
      AND COALESCE(
          private_data->>'result_url',
          data::jsonb->'data'->'task_result'->'videos'->0->>'url',
          data::jsonb->>'video_url',
          data::jsonb->>'url'
      ) != ''
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
        COALESCE(
            private_data->>'result_url',
            CASE
                WHEN data::text != '{}' AND data::text != '' THEN
                    COALESCE(
                        data::jsonb->'task'->>'video_url',
                        data::jsonb->>'video_url',
                        data::jsonb->>'url'
                    )
            END
        ) as result_url
    FROM tasks
    WHERE platform = '60'
      AND status = 'SUCCESS'
      AND COALESCE(
          private_data->>'result_url',
          data::jsonb->'task'->>'video_url',
          data::jsonb->>'video_url',
          data::jsonb->>'url'
      ) IS NOT NULL
      AND COALESCE(
          private_data->>'result_url',
          data::jsonb->'task'->>'video_url',
          data::jsonb->>'video_url',
          data::jsonb->>'url'
      ) != ''
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
        COALESCE(
            private_data->>'result_url',
            CASE
                WHEN data::text != '{}' AND data::text != '' THEN
                    COALESCE(
                        data::jsonb->'content'->>'video_url',
                        data::jsonb->>'video_url',
                        data::jsonb->>'url'
                    )
            END
        ) as result_url
    FROM tasks
    WHERE platform = '54'
      AND status = 'SUCCESS'
      AND COALESCE(
          private_data->>'result_url',
          data::jsonb->'content'->>'video_url',
          data::jsonb->>'video_url',
          data::jsonb->>'url'
      ) IS NOT NULL
      AND COALESCE(
          private_data->>'result_url',
          data::jsonb->'content'->>'video_url',
          data::jsonb->>'video_url',
          data::jsonb->>'url'
      ) != ''
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
