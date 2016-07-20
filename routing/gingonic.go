package routing

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ginRouter struct {
	router *gin.Engine
}

func (g ginRouter) Handler() http.Handler {
	return g.Handler()
}

func (g ginRouter) Handle(protocol, route string, handler HandlerFunc) {
	wrappedCallback := func(c *gin.Context) {
		params := map[string]string{}
		for _, p := range c.Params {
			params[p.Key] = p.Value
		}

		handler(c.Writer, c.Request, params)
	}

	g.router.Handle(protocol, route, wrappedCallback)
}

//New creates a new api2go router to use with the gin framework
func New(g *gin.Engine) Routeable {
	return &ginRouter{router: g}
}
