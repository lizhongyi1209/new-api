-- Backfill video URLs for old logs without task_id
-- Match by user_id, created_at timestamp proximity, and model compatibility

BEGIN;

-- Create a temporary table to store matched log-task pairs
CREATE TEMP TABLE log_task_matches (
    log_id INTEGER,
    task_id TEXT,
    result_url TEXT
);

-- Match Kling logs to tasks by user_id and timestamp proximity (within 60 seconds)
-- Use DISTINCT ON to ensure one-to-one matching
INSERT INTO log_task_matches (log_id, task_id, result_url)
SELECT DISTINCT ON (l.id)
    l.id as log_id,
    t.task_id,
    t.private_data->>'result_url' as result_url
FROM logs l
INNER JOIN tasks t ON
    l.user_id = t.user_id
    AND t.platform = '50'
    AND t.status = 'SUCCESS'
    AND t.private_data->>'result_url' IS NOT NULL
    AND t.private_data->>'result_url' != ''
    -- Match by time: task created within 60 seconds of log
    AND ABS(t.created_at - l.created_at) <= 60
WHERE l.type = 2
  AND l.other::jsonb ? 'is_task'
  AND (l.other::jsonb->>'task_id' IS NULL OR l.other::jsonb->>'task_id' = '')
  AND (l.other::jsonb->>'video_url' IS NULL OR l.other::jsonb->>'video_url' = '')
  AND l.model_name LIKE '%kling%'
ORDER BY l.id, ABS(t.created_at - l.created_at) ASC;

-- Update logs with matched video URLs
UPDATE logs
SET other = jsonb_set(
    other::jsonb,
    '{video_url}',
    to_jsonb(m.result_url),
    true
)::text
FROM log_task_matches m
WHERE logs.id = m.log_id;

-- Show statistics
SELECT
    COUNT(*) as total_matched,
    COUNT(DISTINCT log_id) as unique_logs_updated,
    COUNT(DISTINCT task_id) as unique_tasks_used
FROM log_task_matches;

DROP TABLE log_task_matches;

COMMIT;
