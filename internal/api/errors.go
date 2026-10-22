package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
	generatedapi "github.com/hedgehog-pm/service-task-schemas/internal/generated/api"
)

// HTTPErrorHandler скрывает внутренние причины ошибок в ответах API.
func HTTPErrorHandler(ctx *echo.Context, err error) {
	// Не можем изменять заголовки или тело отправленного ответа.
	if isResponseCommitted(ctx) {
		return
	}

	status := http.StatusInternalServerError
	message := "internal error"

	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
		message = "invalid input"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		message = "schema not found"
	default:
		if httpStatus := echo.StatusCode(err); httpStatus != 0 {
			status = httpStatus
		}
	}

	payload := generatedapi.Error{
		Message: message,
	}

	if writeErr := ctx.JSON(status, payload); writeErr != nil {
		echo.DefaultHTTPErrorHandler(false)(ctx, writeErr)
	}
}

func isResponseCommitted(ctx *echo.Context) bool {
	resp, err := echo.UnwrapResponse(ctx.Response())
	return err == nil && resp.Committed
}
