package volcengine

import (
	"encoding/json"
	"testing"
)

// TestDecodeServerFrameParsesFinalPayload 验证服务端最终帧可以被正确解析。
func TestDecodeServerFrameParsesFinalPayload(t *testing.T) {
	body, err := json.Marshal(responsePayload{
		Code:     0,
		Message:  "success",
		Sequence: -1,
		Result: []struct {
			Text       string  `json:"text"`
			Confidence float64 `json:"confidence"`
			Utterances []struct {
				Text string `json:"text"`
			} `json:"utterances"`
		}{
			{
				Text:       "识别成功",
				Confidence: 0.98,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal response payload: %v", err)
	}

	header := []byte{
		(protocolVersion << 4) | headerSize,
		(messageTypeFullServer << 4) | flagNegativeSequence,
		(serializationJSON << 4) | compressionGzip,
		0,
	}
	frame := appendFramePayload(header, gzipData(body))

	payload, isFinal, err := decodeServerFrame(frame)
	if err != nil {
		t.Fatalf("decodeServerFrame returned error: %v", err)
	}
	if !isFinal {
		t.Fatalf("expected final frame")
	}
	if got := extractResultText(payload); got != "识别成功" {
		t.Fatalf("expected text 识别成功, got %q", got)
	}
	if got := extractConfidence(payload); got != 0.98 {
		t.Fatalf("expected confidence 0.98, got %v", got)
	}
}
