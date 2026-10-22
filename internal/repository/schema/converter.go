package schema

import (
	"github.com/samber/lo"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
	sqlcgen "github.com/hedgehog-pm/service-task-schemas/internal/generated/sqlc/schema"
)

func SchemaToDomain(row sqlcgen.Schema) domain.Schema {
	return domain.Schema{
		ID:              SchemaToDomainID(row),
		Parent:          ParentSchemaToDomainID(row),
		OwnSchema:       row.OwnSchema,
		EffectiveSchema: row.EffectiveSchema,
	}
}

func DomainToInsertSchemaParams(
	model domain.Schema,
) sqlcgen.InsertSchemaParams {
	params := sqlcgen.InsertSchemaParams{
		Slug:            model.ID.Slug,
		Version:         int32(model.ID.Version),
		OwnSchema:       model.OwnSchema,
		EffectiveSchema: model.EffectiveSchema,
	}

	if model.Parent != nil {
		params.ParentSlug = new(model.Parent.Slug)
		params.ParentVersion = new(int32(model.Parent.Version))
	}

	return params
}

func SchemaToDomainID(row sqlcgen.Schema) domain.ID {
	return domain.ID{
		Slug:    row.Slug,
		Version: int(row.Version),
	}
}

func ParentSchemaToDomainID(row sqlcgen.Schema) *domain.ID {
	if row.ParentSlug == nil || row.ParentVersion == nil {
		return nil
	}

	return lo.EmptyableToPtr(domain.ID{
		Slug:    *row.ParentSlug,
		Version: int(*row.ParentVersion),
	})
}
