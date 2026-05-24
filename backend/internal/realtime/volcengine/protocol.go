package volcengine

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	protocolVersion1 byte = 0x10
	protocolHeader4  byte = 0x01

	messageTypeFullClient  byte = 0x10
	messageTypeAudioClient byte = 0x20
	messageTypeFullServer  byte = 0x90
	messageTypeAudioServer byte = 0xB0
	messageTypeError       byte = 0xF0

	messageFlagWithEvent byte = 0x04
	messageFlagPositiveSeq byte = 0x01
	messageFlagNegativeSeq byte = 0x03

	serializationRaw  byte = 0x00
	serializationJSON byte = 0x10
)

// protocolMessage 描述火山实时语音协议中的一帧消息。
type protocolMessage struct {
	MessageType   byte
	Flag          byte
	Serialization byte
	Event         int32
	SessionID     string
	ConnectID     string
	Sequence      int32
	ErrorCode     uint32
	Payload       []byte
}

// marshalProtocolMessage 将协议消息编码为二进制帧。
func marshalProtocolMessage(msg protocolMessage) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(protocolVersion1 | protocolHeader4)
	buf.WriteByte(msg.MessageType | msg.Flag)
	buf.WriteByte(msg.Serialization)
	buf.WriteByte(0)

	if msg.Flag&messageFlagWithEvent == messageFlagWithEvent {
		if err := binary.Write(&buf, binary.BigEndian, msg.Event); err != nil {
			return nil, fmt.Errorf("write realtime event failed: %w", err)
		}
		if shouldCarrySessionID(msg.Event) {
			if err := writeSizedString(&buf, msg.SessionID); err != nil {
				return nil, err
			}
		}
	}

	if err := binary.Write(&buf, binary.BigEndian, uint32(len(msg.Payload))); err != nil {
		return nil, fmt.Errorf("write realtime payload size failed: %w", err)
	}
	if len(msg.Payload) > 0 {
		if _, err := buf.Write(msg.Payload); err != nil {
			return nil, fmt.Errorf("write realtime payload failed: %w", err)
		}
	}
	return buf.Bytes(), nil
}

// unmarshalProtocolMessage 将服务端返回帧解析为协议消息。
func unmarshalProtocolMessage(frame []byte) (*protocolMessage, error) {
	if len(frame) < 8 {
		return nil, fmt.Errorf("realtime frame is too short")
	}

	buf := bytes.NewBuffer(frame)
	header0, _ := buf.ReadByte()
	header1, _ := buf.ReadByte()
	header2, _ := buf.ReadByte()
	_, _ = buf.ReadByte()

	if header0>>4 != protocolVersion1>>4 {
		return nil, fmt.Errorf("unsupported realtime protocol version: %d", header0>>4)
	}

	msg := &protocolMessage{
		MessageType:   header1 & 0xF0,
		Flag:          header1 & 0x0F,
		Serialization: header2 & 0xF0,
	}

	if msg.MessageType == messageTypeError {
		if err := binary.Read(buf, binary.BigEndian, &msg.ErrorCode); err != nil {
			return nil, fmt.Errorf("read realtime error code failed: %w", err)
		}
	}

	if shouldCarrySequence(msg.MessageType, msg.Flag) {
		if err := binary.Read(buf, binary.BigEndian, &msg.Sequence); err != nil {
			return nil, fmt.Errorf("read realtime sequence failed: %w", err)
		}
	}

	if msg.Flag&messageFlagWithEvent == messageFlagWithEvent {
		if err := binary.Read(buf, binary.BigEndian, &msg.Event); err != nil {
			return nil, fmt.Errorf("read realtime event failed: %w", err)
		}
		if shouldCarrySessionID(msg.Event) {
			sessionID, err := readSizedString(buf)
			if err != nil {
				return nil, err
			}
			msg.SessionID = sessionID
		}
		if shouldCarryConnectID(msg.Event) {
			connectID, err := readSizedString(buf)
			if err != nil {
				return nil, err
			}
			msg.ConnectID = connectID
		}
	}

	var payloadSize uint32
	if err := binary.Read(buf, binary.BigEndian, &payloadSize); err != nil {
		return nil, fmt.Errorf("read realtime payload size failed: %w", err)
	}
	if payloadSize > uint32(buf.Len()) {
		return nil, fmt.Errorf("invalid realtime payload size: %d", payloadSize)
	}
	if payloadSize > 0 {
		msg.Payload = buf.Next(int(payloadSize))
	}
	return msg, nil
}

// shouldCarrySessionID 判断当前事件是否需要携带 session_id 字段。
func shouldCarrySessionID(event int32) bool {
	switch event {
	case 1, 2, 50, 51, 52:
		return false
	default:
		return true
	}
}

// shouldCarryConnectID 判断当前事件是否需要携带 connect_id 字段。
func shouldCarryConnectID(event int32) bool {
	switch event {
	case 50, 51, 52:
		return true
	default:
		return false
	}
}

// shouldCarrySequence 判断当前帧是否包含 sequence 字段。
func shouldCarrySequence(messageType byte, flag byte) bool {
	if messageType != messageTypeAudioClient && messageType != messageTypeAudioServer {
		return false
	}
	return flag == messageFlagPositiveSeq || flag == messageFlagNegativeSeq
}

// writeSizedString 按协议要求写入长度前缀字符串。
func writeSizedString(buf *bytes.Buffer, value string) error {
	if err := binary.Write(buf, binary.BigEndian, uint32(len(value))); err != nil {
		return fmt.Errorf("write realtime string size failed: %w", err)
	}
	if len(value) == 0 {
		return nil
	}
	if _, err := buf.WriteString(value); err != nil {
		return fmt.Errorf("write realtime string body failed: %w", err)
	}
	return nil
}

// readSizedString 读取协议中的长度前缀字符串。
func readSizedString(buf *bytes.Buffer) (string, error) {
	var size uint32
	if err := binary.Read(buf, binary.BigEndian, &size); err != nil {
		return "", fmt.Errorf("read realtime string size failed: %w", err)
	}
	if size == 0 {
		return "", nil
	}
	if size > uint32(buf.Len()) {
		return "", fmt.Errorf("invalid realtime string size: %d", size)
	}
	return string(buf.Next(int(size))), nil
}
