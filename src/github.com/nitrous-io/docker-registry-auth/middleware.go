package main

import (
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/justinas/alice"
)

// logging is a middleware that logs the HTTP request and response.
func logging(logHeaders bool, logFormData bool) alice.Constructor {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			fields := logrus.Fields{}
			fields["method"] = r.Method
			fields["path"] = r.URL.String()
			if logHeaders {
				fields["headers"] = r.Header
			}
			if logFormData {
				r.ParseMultipartForm(32 << 20) // defaultMaxMemory from net/http; ParseMultipartForm is idempotent
				fields["params"] = r.PostForm
			}
			logrus.WithFields(fields).Info("Request received")

			logger := &responseLogger{ResponseWriter: w}
			h.ServeHTTP(logger, r)

			delete(fields, "headers")
			delete(fields, "params")
			fields["status"] = logger.Status()
			fields["duration"] = time.Since(start)
			logrus.WithFields(fields).Info("Request completed")
		})
	}
}

type responseLogger struct {
	http.ResponseWriter
	status int
}

func (r *responseLogger) WriteHeader(s int) {
	r.ResponseWriter.WriteHeader(s)
	r.status = s
}

func (r *responseLogger) Status() int {
	return r.status
}
