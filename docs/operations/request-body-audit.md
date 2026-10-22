# Request body audit snapshots

Relay, task, async image, and Midjourney submissions write a bounded request
snapshot into the administrator-only `other.admin_info.request_snapshot`
field of the usage or error log.

The snapshot records the request method, path, content type, body size, and
scalar fields. Nested JSON fields use paths such as
`messages[0].content`. The following values are never persisted:

- credentials, authorization values, cookies, signatures, and private keys;
- Base64 or data-URI payloads;
- uploaded files and other binary media;
- strings above 4 KiB;
- snapshots above 32 KiB or 128 fields.

Omitted fields retain their name, byte length, and an omission reason. A hash
may be retained for non-secret Base64 or oversized values so repeated failures
can be correlated without storing the payload.

## PostgreSQL query

```sql
SELECT id,
       request_id,
       created_at,
       model_name,
       "group",
       other::jsonb->>'request_path' AS request_path,
       other::jsonb->'admin_info'->'request_snapshot' AS request_snapshot,
       content
FROM logs
WHERE other IS NOT NULL
  AND NULLIF(other, '')::jsonb->'admin_info' ? 'request_snapshot'
ORDER BY id DESC
LIMIT 100;
```

For one request, add `AND request_id = '...'`. The snapshot is administrator
metadata and is removed from ordinary user log projections.
