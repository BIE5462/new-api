package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVideoRouterRegistersKlingMotionControlRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetVideoRouter(engine)

	assertRouteRegistered(t, engine, http.MethodPost, "/kling/v1/videos/motion-control")
	assertRouteRegistered(t, engine, http.MethodGet, "/kling/v1/videos/motion-control/:task_id")
}

func assertRouteRegistered(t *testing.T, engine *gin.Engine, method, path string) {
	t.Helper()
	for _, route := range engine.Routes() {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("route %s %s not registered", method, path)
}
