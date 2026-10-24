-- name: GetEffectiveSchema :one
SELECT effective_schema FROM schemas
WHERE slug = $1 AND version = $2;

-- name: GetSchemaLineage :many
WITH RECURSIVE lineage AS (
    SELECT s.slug, s.version, s.parent_slug, s.parent_version, 0 AS depth
    FROM schemas AS s
    WHERE s.slug = @slug AND s.version = @version::bigint

    UNION ALL

    SELECT parent.slug, parent.version, parent.parent_slug, parent.parent_version, lineage.depth + 1
    FROM schemas AS parent
    JOIN lineage ON parent.slug = lineage.parent_slug AND parent.version = lineage.parent_version
)
SELECT lineage.slug, lineage.version FROM lineage
ORDER BY lineage.depth;

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
