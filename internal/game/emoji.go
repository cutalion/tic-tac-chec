package game

// ReactionEmojis is the ordered list of allowed emoji reactions.
// This is the single source of truth — the frontend reads it via template injection.
var ReactionEmojis = []string{
	"👍", "😂", "😮", "😤", "🤔", "👋", "🏆", "🎯",
	"🪤", "😈", "🫡", "💀", "🤯", "❤️", "🔥", "😅",
	"🙈", "🫠", "🥶", "💪", "🧠", "⚡", "🎉", "🫶",
}

var allowedReactions = func() map[string]bool {
	m := make(map[string]bool, len(ReactionEmojis))
	for _, e := range ReactionEmojis {
		m[e] = true
	}
	return m
}()

func ValidReaction(s string) bool {
	return allowedReactions[s]
}
