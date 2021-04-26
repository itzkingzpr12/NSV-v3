package routes

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/controllers"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"go.uber.org/zap"
)

// LoggingMiddleware logs the incoming HTTP request & its duration.
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			rqID := uuid.New()

			start := time.Now()

			body, _ := ioutil.ReadAll(r.Body)
			rdr := ioutil.NopCloser(bytes.NewBuffer(body))
			r.Body = rdr

			ctx := r.Context()

			if r.Header.Get("Request-ID") != "" {
				ctx = logging.AddValues(ctx,
					zap.String("request_id", r.Header.Get("Request-ID")),
				)
			} else {
				ctx = logging.AddValues(ctx,
					zap.String("request_id", rqID.String()),
				)
			}

			vars := mux.Vars(r)
			for key, val := range vars {
				ctx = logging.AddValues(ctx,
					zap.String(key, val),
				)
			}

			if r.Header.Get("Guild") != "" {
				ctx = logging.AddValues(ctx, zap.String("guild", r.Header.Get("Guild")))
			}

			r = r.WithContext(ctx)
			wrapped := wrapResponseWriter(w)

			ctx = logging.AddValues(ctx,
				zap.String("proto", r.Proto),
				zap.String("method", r.Method),
				zap.String("path", r.URL.EscapedPath()),
				zap.Any("query_params", r.URL.Query()),
				zap.String("remote_address", r.RemoteAddr),
				zap.String("request_body", string(body)),
			)

			defer func() {
				if err := recover(); err != nil {
					ctx := logging.AddValues(ctx,
						zap.Int("status", wrapped.status),
						zap.Float64("duration", float64(time.Since(start).Nanoseconds())/1e6),
						zap.Any("error", err),
						zap.String("trace", string(debug.Stack())),
					)
					logger := logging.Logger(ctx)
					logger.Error("panic_log")

					controllers.Error(ctx, wrapped, "panic", err.(error), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(wrapped, r)

			if r.URL.Path == "/nitrado-server-manager-v2/status" {
				return
			}

			if wrapped.status >= 200 && wrapped.status <= 399 {
				ctx = logging.AddValues(ctx,
					zap.Int("status", wrapped.status),
					zap.Float64("duration", float64(time.Since(start).Nanoseconds())/1e6),
					//zap.Any("response_body", string(wrapped.message)),
				)

				cacheHeader := wrapped.Header().Get("X-Cache")

				if cacheHeader == "HIT" {
					ctx = logging.AddValues(ctx, zap.Bool("from_cache", true))
				} else {
					ctx = logging.AddValues(ctx, zap.Bool("from_cache", false))
				}
			} else {
				ctx = logging.AddValues(ctx,
					zap.Int("status", wrapped.status),
					zap.Float64("duration", float64(time.Since(start).Nanoseconds())/1e6),
					zap.Any("response_body", string(wrapped.message)),
				)
			}

			logger := logging.Logger(ctx)
			logger.Info("access_log")
		}

		return http.HandlerFunc(fn)
	}
}
