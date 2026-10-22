package api

import (
	"net/http"

	"github.com/labstack/echo/v5"

	generatedapi "github.com/hedgehog-pm/service-task-schemas/internal/generated/api"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/findschemas"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/getschema"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/listschemaversions"
)

type Server struct {
	getSchema    *getschema.Usecase
	listVersions *listschemaversions.Usecase
	findSchemas  *findschemas.Usecase
}

func NewServer(
	getSchema *getschema.Usecase,
	listVersions *listschemaversions.Usecase,
	findSchemas *findschemas.Usecase,
) *Server {
	return &Server{
		getSchema:    getSchema,
		listVersions: listVersions,
		findSchemas:  findSchemas,
	}
}

func (h *Server) FindSchemas(
	ctx *echo.Context,
	params generatedapi.FindSchemasParams,
) error {
	schemas, err := h.findSchemas.Do(ctx.Request().Context(), params.Fields)
	if err != nil {
		return err
	}

	items := make([]generatedapi.SchemaMatch, len(schemas))
	for i, schema := range schemas {
		items[i] = generatedapi.SchemaMatch{
			Slug:    schema.ID.Slug,
			Version: schema.ID.Version,
			Schema:  schema.EffectiveSchema,
		}
	}

	return ctx.JSON(http.StatusOK, generatedapi.SchemaMatches{Items: items})
}

func (h *Server) ListSchemaVersions(ctx *echo.Context, slug string) error {
	versions, err := h.listVersions.Do(ctx.Request().Context(), slug)
	if err != nil {
		return err
	}

	return ctx.JSON(
		http.StatusOK,
		generatedapi.SchemaVersions{Versions: versions},
	)
}

func (h *Server) GetSchema(
	ctx *echo.Context,
	slug string,
	version int,
) error {
	document, err := h.getSchema.Do(ctx.Request().Context(), slug, version)
	if err != nil {
		return err
	}

	return ctx.JSONBlob(http.StatusOK, document)
}
