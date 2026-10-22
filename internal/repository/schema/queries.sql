-- name: GetEffectiveSchema :one
SELECT effective_schema FROM schemas
WHERE slug = $1 AND version = $2;

-- name: ListSchemaVersions :many
SELECT version FROM schemas
WHERE slug = $1
ORDER BY version;

-- name: FindSchemasByFields :many
SELECT s.* FROM schemas AS s
WHERE NOT EXISTS (
    SELECT 1
    FROM unnest(@fields::text[]) AS requested(field_name)
    WHERE NOT EXISTS (
        SELECT 1
        FROM json_array_elements(s.effective_schema -> 'allOf') AS part(schema_part)
        WHERE (part.schema_part::jsonb -> 'properties') ? requested.field_name
    )
)
ORDER BY s.slug, s.version;

-- name: FindSchemas :many
SELECT * FROM schemas
WHERE (slug, version) IN (
    SELECT unnest(@slugs::text[]), unnest(@versions::bigint[])
);

-- name: InsertSchema :execrows
INSERT INTO schemas (slug, version, parent_slug, parent_version, own_schema, effective_schema)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (slug, version) DO NOTHING;
