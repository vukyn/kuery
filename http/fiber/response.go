package http

import (
	"net/http"

	pkgBase "github.com/vukyn/kuery/http/base"
	pkgErr "github.com/vukyn/kuery/http/errors"

	"github.com/gofiber/fiber/v2"
)

func OK(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(pkgBase.Response{
		Code:    fiber.StatusOK,
		Message: "OK",
		Data:    data,
	})
}

func Err(c *fiber.Ctx, err error) error {
	switch err := err.(type) {
	case pkgErr.Error:
		switch err.Status() {
		case http.StatusUnauthorized:
			return Unauthorized(c)
		default:
			return c.Status(err.Status()).JSON(pkgBase.Response{
				Code:    err.Status(),
				Message: err.Error(),
			})
		}
	default:
		return c.Status(http.StatusInternalServerError).JSON(pkgBase.Response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}
}

func Unauthorized(c *fiber.Ctx) error {
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
		"error": "Unauthorized",
	})
}
