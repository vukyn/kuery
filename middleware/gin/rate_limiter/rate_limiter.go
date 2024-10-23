package ratelimiter

import (
	"github.com/didip/tollbooth/v7"
	"github.com/gin-gonic/gin"
)

func LimitPerSecond(max int) gin.HandlerFunc {
	lmt := tollbooth.NewLimiter(float64(max), nil)
	return func(c *gin.Context) {
		httpError := tollbooth.LimitByRequest(lmt, c.Writer, c.Request)
		if httpError != nil {
			c.Data(httpError.StatusCode, lmt.GetMessageContentType(), []byte(httpError.Message))
			c.Abort()
		} else {
			c.Next()
		}
	}
}
