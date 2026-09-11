package http

import (
	pkgHealth "github.com/vukyn/kuery/http/health"

	"github.com/gofiber/fiber/v3"
)

// Health is the Fiber v3 twin of http/fiber.Health — same collector, same
// unwrapped body, only the Ctx type differs. Both wrappers are this thin
// because all the logic lives in the framework-free health package; there is
// no second copy to keep in sync, which is how the v2/v3 pair here avoids the
// drift that auth/authv3 accumulated.
//
//	collector := pkgHealth.New("github.com/vukyn/kuery", "github.com/gofiber/fiber/v3")
//	app.Get("/__version", pkgHttp.Health(collector))
//
// Read the health package doc before wiring this up — in particular why it must
// never gain a database dependency, and why nothing may poll it.
func Health(collector *pkgHealth.Collector) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(collector.Collect())
	}
}
