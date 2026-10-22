-- +goose Up
CREATE TABLE schemas (
    slug             TEXT    NOT NULL,
    version          INTEGER NOT NULL,
    parent_slug      TEXT,
    parent_version   INTEGER,
    own_schema       JSONB   NOT NULL,
    effective_schema JSON    NOT NULL,

    PRIMARY KEY (slug, version),
    CONSTRAINT schemas_version_positive CHECK (version >= 1),
    CONSTRAINT schemas_parent_version_positive CHECK (
        parent_version IS NULL OR parent_version >= 1
    ),
    CONSTRAINT schemas_parent_pair CHECK (
        (parent_slug IS NULL AND parent_version IS NULL)
        OR (parent_slug IS NOT NULL AND parent_version IS NOT NULL)
    ),
    CONSTRAINT schemas_parent_fk FOREIGN KEY (parent_slug, parent_version)
        REFERENCES schemas (slug, version)
        -- FK отложен, чтобы seed мог вставлять схемы в любом порядке внутри одной транзакции.
        -- Сугубо для удобства
        DEFERRABLE INITIALLY DEFERRED
);

COMMENT ON COLUMN schemas.own_schema IS
    'Собственная часть схемы без полей предков.';
COMMENT ON COLUMN schemas.effective_schema IS
    'Итоговая схема с полями всех предков.';

-- Готовые схемы потомков остаются корректными, пока исходные версии неизменяемы.
-- +goose StatementBegin
CREATE FUNCTION reject_schema_change() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'published schema %.v% is immutable', OLD.slug, OLD.version
        USING ERRCODE = '23514';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER schemas_immutable
BEFORE UPDATE OR DELETE ON schemas
FOR EACH ROW EXECUTE FUNCTION reject_schema_change();
