package http

import (
	"log"
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
		switch {
		case err.Status() == http.StatusUnauthorized:
			return Unauthorized(c)
		case err.Status() >= http.StatusInternalServerError:
			// 5xx carries internal detail (DB driver errors, stack/parse details).
			// Log the real message server-side but return a generic body so
			// internals never leak to the client.
			log.Printf("[http] %d %s %s: %v", err.Status(), c.Method(), c.OriginalURL(), err.Error())
			return c.Status(err.Status()).JSON(pkgBase.Response{
				Code:    err.Status(),
				Message: "internal server error",
			})
		default:
			return c.Status(err.Status()).JSON(pkgBase.Response{
				Code:    err.Status(),
				Message: err.Error(),
			})
		}
	default:
		// Unexpected/internal error: log the real detail server-side (visible in
		// the service logs) but return a generic message so internals — parse
		// errors, stack details, secrets — never leak to the client.
		log.Printf("[http] 500 %s %s: %v", c.Method(), c.OriginalURL(), err)
		return c.Status(http.StatusInternalServerError).JSON(pkgBase.Response{
			Code:    http.StatusInternalServerError,
			Message: "internal server error",
		})
	}
}

func Unauthorized(c *fiber.Ctx) error {
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
		"error": "Unauthorized",
	})
}

func Forbidden(c *fiber.Ctx) error {
	return c.Status(http.StatusForbidden).JSON(fiber.Map{
		"error": "Forbidden",
	})
}
