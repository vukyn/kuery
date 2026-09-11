package http

import (
	pkgHealth "github.com/vukyn/kuery/http/health"

	"github.com/gofiber/fiber/v2"
)

// Health serves a version report built by the collector. Mount it once, on a
// route the service picks:
//
//	collector := pkgHealth.New("github.com/vukyn/kuery", "github.com/gofiber/fiber/v2")
//	app.Get("/__version", pkgHttp.Health(collector))
//
// The body is the report itself, NOT the base.Response{code,message,data}
// envelope the rest of this package returns. That is deliberate: this endpoint
// is read by a human with curl and by tooling that knows nothing about the
// platform's envelope, and status belongs at the top level where jq finds it
// without a .data hop.
//
// Read the health package doc before wiring this up — in particular why it must
// never gain a database dependency, and why nothing may poll it.
func Health(collector *pkgHealth.Collector) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(collector.Collect())
	}
}
