package recover

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/vukyn/kuery/log"
)

func NewFiberRecover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		// Catch panics
		defer func() {
			if r := recover(); r != nil {
				switch r := r.(type) {
				case error:
					log.New().Errorf("Panic recovered: %v", r)
				case string:
					log.New().Errorf("Panic recovered: %v", errors.New(r))
				default:
					log.New().Errorf("Panic recovered: %v", r)
				}
				err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": "Something went wrong, please try again later!",
				})
			}
		}()
		return c.Next()
	}
}
