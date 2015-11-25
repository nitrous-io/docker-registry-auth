package main

import (
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
)

// logging is a middleware that logs the HTTP request and response.
func logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		fields := logrus.Fields{}
		fields["method"] = r.Method
		fields["path"] = r.URL.String()
		logrus.WithFields(fields).Info("Request received")

		logger := &responseLogger{ResponseWriter: w}
		h.ServeHTTP(logger, r)

		fields["status"] = logger.Status()
		fields["duration"] = time.Since(start)
		logrus.WithFields(fields).Info("Request completed")
	})
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
