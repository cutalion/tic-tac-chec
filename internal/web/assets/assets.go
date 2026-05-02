package assets

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/web/config"
)

const (
	analyticsEnabledPlaceholder = "__ANALYTICS_ENABLED__"
	posthogKeyPlaceholder       = "__POSTHOG_KEY__"
	posthogHostPlaceholder      = "__POSTHOG_HOST__"
	reactionEmojiPlaceholder    = "__REACTION_EMOJIS__"
)

var reactionEmojisJSON = func() string {
	b, _ := json.Marshal(game.ReactionEmojis)
	return string(b)
}()

func WriteTemplatedAsset(w http.ResponseWriter, contentType string, raw []byte, cfg config.Analytics) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")

	content := string(raw)
	content = strings.ReplaceAll(content, analyticsEnabledPlaceholder, strconv.FormatBool(cfg.Enabled))
	content = strings.ReplaceAll(content, posthogKeyPlaceholder, strconv.Quote(cfg.PostHog.Key))
	content = strings.ReplaceAll(content, posthogHostPlaceholder, strconv.Quote(cfg.PostHog.Host))
	content = strings.ReplaceAll(content, reactionEmojiPlaceholder, reactionEmojisJSON)
	_, _ = io.WriteString(w, content)
}
