package game

import "testing"

func TestValidReaction(t *testing.T) {
	valid := []string{
		"👍", "😂", "😮", "😤", "🤔", "👋",
		"🏆", "🎯", "🪤", "😈", "🫡", "💀",
		"🤯", "❤️", "🔥", "😅",
	}

	for _, emoji := range valid {
		if !ValidReaction(emoji) {
			t.Errorf("expected %q to be valid", emoji)
		}
	}

	invalid := []string{
		"",
		"hello",
		"😂hello",
		"<script>",
		"💩",
		"❤️❤️",
		" ",
	}

	for _, s := range invalid {
		if ValidReaction(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}
