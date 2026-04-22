package eino

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestExtractMessageTextIgnoresReasoning 验证普通聊天文本不会拼接模型的推理内容。
func TestExtractMessageTextIgnoresReasoning(t *testing.T) {
	t.Parallel()

	message := &schema.Message{
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "辛苦了。先休息一下也没关系的。",
			},
		},
		ReasoningContent: "这段 reasoning 也不应该被拼进去。",
	}

	got := extractMessageText(message)
	want := "辛苦了。先休息一下也没关系的。"
	if got != want {
		t.Fatalf("unexpected message text: got %q want %q", got, want)
	}
}

// TestSanitizeVisibleMessageContentRemovesThinkBlock 验证正文内嵌 think 标签时也会被剥离。
func TestSanitizeVisibleMessageContentRemovesThinkBlock(t *testing.T) {
	t.Parallel()

	got := sanitizeVisibleMessageContent("<think>这里是思维链</think>\n\n辛苦了，先休息一会儿。")
	want := "辛苦了，先休息一会儿。"
	if got != want {
		t.Fatalf("unexpected sanitized content: got %q want %q", got, want)
	}
}
