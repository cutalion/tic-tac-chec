package router

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5/middleware"
)

type redactingFormatter struct {
	inner *middleware.DefaultLogFormatter
}

func (f *redactingFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	rc := *r
	rc.RequestURI = redactURI(r.RequestURI)

	return f.inner.NewLogEntry(&rc)
}

var RedactingLogFormatter = &redactingFormatter{
	inner: &middleware.DefaultLogFormatter{
		Logger:  log.New(os.Stdout, "", log.LstdFlags),
		NoColor: false,
	},
}

func redactURI(uri string) string {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return uri
	}
	q := u.Query()
	changed := false

	for _, key := range []string{"token"} {
		if q.Get(key) != "" {
			q.Set(key, "[FILTERED]")
			changed = true
		}
	}
	if !changed {
		return uri
	}

	u.RawQuery = q.Encode()
	return u.String()
}
