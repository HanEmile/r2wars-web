package main

import (
	"net/http"
	"os"

	"github.com/gorilla/handlers"
)

// Defines a middleware containing a logfile
//
// This is done to combine gorilla/handlers with gorilla/mux middlewares to
// just use r.Use(logger.Middleware) once instead of adding this to all
// handlers manually (Yes, I'm really missing macros in Go...)
type loggingMiddleware struct {
	logFile *os.File
}

func (l *loggingMiddleware) Middleware(next http.Handler) http.Handler {
	return handlers.LoggingHandler(l.logFile, next)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := globalState.sessions.Get(r, "session")
		username := session.Values["username"]

		if username == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
