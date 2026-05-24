package volcengine

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	appconfig "makejob-backend/internal/config"
)

// TestNewClientRejectsDisabledConfig 验证未开启实时语音能力时会返回明确配置错误。
func TestNewClientRejectsDisabledConfig(t *testing.T) {
	_, err := NewClient(appconfig.VolcRealtimeDialogConfig{})
	if err == nil {
		t.Fatal("expected disabled realtime config error")
	}
	if !strings.Contains(err.Error(), "enabled=true") {
		t.Fatalf("expected enabled hint in error, got %v", err)
	}
}

// TestNewClientRequiresOfficialCredentials 验证实时面试必须提供 app_id 和 access_token。
func TestNewClientRequiresOfficialCredentials(t *testing.T) {
	_, err := NewClient(appconfig.VolcRealtimeDialogConfig{
		Enabled: true,
		AppID:   "appid-only",
	})
	if err == nil {
		t.Fatal("expected missing access_token error")
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected access_token hint in error, got %v", err)
	}
}

// TestNewClientNormalizesFixedAppKey 验证客户端会强制使用文档要求的固定 AppKey。
func TestNewClientNormalizesFixedAppKey(t *testing.T) {
	client, err := NewClient(appconfig.VolcRealtimeDialogConfig{
		Enabled:     true,
		AppID:       "demo-app",
		AccessToken: "demo-token",
		AppKey:      "wrong-app-key",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client.cfg.AppKey != defaultRealtimeDialogAppKey {
		t.Fatalf("expected fixed app key %q, got %q", defaultRealtimeDialogAppKey, client.cfg.AppKey)
	}
}

// TestUnmarshalProtocolMessageReadsAudioSequence 验证带 sequence 的音频服务端帧不会再把 payload 读错位。
func TestUnmarshalProtocolMessageReadsAudioSequence(t *testing.T) {
	var frame bytes.Buffer
	frame.WriteByte(protocolVersion1 | protocolHeader4)
	frame.WriteByte(messageTypeAudioServer | messageFlagPositiveSeq)
	frame.WriteByte(serializationRaw)
	frame.WriteByte(0)
	if err := binary.Write(&frame, binary.BigEndian, int32(7)); err != nil {
		t.Fatalf("write sequence: %v", err)
	}
	payload := []byte{1, 2, 3, 4}
	if err := binary.Write(&frame, binary.BigEndian, uint32(len(payload))); err != nil {
		t.Fatalf("write payload size: %v", err)
	}
	if _, err := frame.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	msg, err := unmarshalProtocolMessage(frame.Bytes())
	if err != nil {
		t.Fatalf("unmarshalProtocolMessage returned error: %v", err)
	}
	if msg.Sequence != 7 {
		t.Fatalf("expected sequence 7, got %d", msg.Sequence)
	}
	if !bytes.Equal(msg.Payload, payload) {
		t.Fatalf("expected payload %v, got %v", payload, msg.Payload)
	}
}
