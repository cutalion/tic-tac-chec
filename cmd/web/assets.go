package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	analyticsEnabledPlaceholder = "__ANALYTICS_ENABLED__"
	posthogKeyPlaceholder       = "__POSTHOG_KEY__"
	posthogHostPlaceholder      = "__POSTHOG_HOST__"
)

func writeTemplatedAsset(w http.ResponseWriter, contentType string, raw []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")

	content := string(raw)
	content = strings.ReplaceAll(content, analyticsEnabledPlaceholder, strconv.FormatBool(analyticsConfig.Enabled))
	content = strings.ReplaceAll(content, posthogKeyPlaceholder, strconv.Quote(analyticsConfig.PostHogKey))
	content = strings.ReplaceAll(content, posthogHostPlaceholder, strconv.Quote(analyticsConfig.PostHogHost))
	_, _ = io.WriteString(w, content)
}
