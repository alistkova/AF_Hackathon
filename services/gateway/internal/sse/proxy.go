package sse

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

func Proxy(rawURL string, logger *slog.Logger) gin.HandlerFunc {
	target, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("upstream request failed", "target", rawURL, "path", r.URL.Path, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":"upstream_unavailable","message":"Сервис временно недоступен"}`))
	}

	return func(c *gin.Context) {
		c.Header("X-Accel-Buffering", "no")

		if c.GetHeader("Accept") == "text/event-stream" {
			c.Header("Cache-Control", "no-cache, no-transform")
			c.Header("Connection", "keep-alive")
		}

		start := time.Now()
		proxy.ServeHTTP(c.Writer, c.Request)
		logger.Debug("proxied request", "path", c.Request.URL.Path, "duration", time.Since(start))
	}
}
