package mock

import (
	"context"
	"sync"
	"time"

	"makejob-backend/internal/asr"
)

// MockASRProvider Mock ASR实现
type MockASRProvider struct{}

// NewMockASRProvider 创建Mock ASR Provider
func NewMockASRProvider() *MockASRProvider {
	return &MockASRProvider{}
}

// Recognize 识别音频数据，返回完整文本
func (m *MockASRProvider) Recognize(ctx context.Context, req asr.RecognizeRequest) (asr.RecognizeResult, error) {
	// 模拟处理延迟
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return asr.RecognizeResult{}, ctx.Err()
	}

	return asr.RecognizeResult{
		Text:       "Go语言的goroutine是轻量级线程，由Go运行时管理",
		Confidence: 0.95,
		Duration:   5.0,
		Language:   req.Language,
	}, nil
}

// StartStream 开始流式识别会话
func (m *MockASRProvider) StartStream(ctx context.Context, engine string, language string) (asr.StreamSession, error) {
	session := &MockStreamSession{
		resultChan: make(chan asr.StreamResult, 10),
		ctx:        ctx,
		language:   language,
	}

	// 启动goroutine模拟流式返回
	go session.simulateStream()

	return session, nil
}

// GetSupportedEngines 获取支持的引擎列表
func (m *MockASRProvider) GetSupportedEngines() []string {
	return []string{"doubao", "xunfei"}
}

// MockStreamSession Mock流式识别会话
type MockStreamSession struct {
	resultChan chan asr.StreamResult
	ctx        context.Context
	language   string
	closed     bool
	mu         sync.Mutex
}

// simulateStream 模拟流式识别过程
func (s *MockStreamSession) simulateStream() {
	// 模拟文本片段
	segments := []string{
		"Go语言的",
		"goroutine",
		"是轻量级线程，",
		"由Go运行时管理",
	}

	for i, segment := range segments {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()

			isFinal := i == len(segments)-1
			s.resultChan <- asr.StreamResult{
				Text:       segment,
				IsFinal:    isFinal,
				Confidence: 0.9 + float64(i)*0.02,
			}
		}
	}
}

// SendAudio 发送音频数据
func (s *MockStreamSession) SendAudio(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	// Mock实现不实际处理音频数据
	return nil
}

// ReceiveText 接收识别结果通道
func (s *MockStreamSession) ReceiveText() <-chan asr.StreamResult {
	return s.resultChan
}

// Close 关闭会话
func (s *MockStreamSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.resultChan)
	}
	return nil
}
