package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"tic-tac-chec/internal/game"
)

const (
	analyticsEnabledPlaceholder  = "__ANALYTICS_ENABLED__"
	posthogKeyPlaceholder        = "__POSTHOG_KEY__"
	posthogHostPlaceholder       = "__POSTHOG_HOST__"
	reactionEmojiPlaceholder     = "__REACTION_EMOJIS__"
)

var reactionEmojisJSON = func() string {
	b, _ := json.Marshal(game.ReactionEmojis)
	return string(b)
}()

func writeTemplatedAsset(w http.ResponseWriter, contentType string, raw []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")

	content := string(raw)
	content = strings.ReplaceAll(content, analyticsEnabledPlaceholder, strconv.FormatBool(analyticsConfig.Enabled))
	content = strings.ReplaceAll(content, posthogKeyPlaceholder, strconv.Quote(analyticsConfig.PostHogKey))
	content = strings.ReplaceAll(content, posthogHostPlaceholder, strconv.Quote(analyticsConfig.PostHogHost))
	content = strings.ReplaceAll(content, reactionEmojiPlaceholder, reactionEmojisJSON)
	_, _ = io.WriteString(w, content)
}
