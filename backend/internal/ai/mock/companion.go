package mock

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"makejob-backend/internal/ai"
)

// MockCompanionAgent Mock陪伴聊天Agent实现
type MockCompanionAgent struct {
	provider ai.AIProvider
}

// NewMockCompanionAgent 创建Mock陪伴聊天Agent
func NewMockCompanionAgent(provider ai.AIProvider) *MockCompanionAgent {
	return &MockCompanionAgent{
		provider: provider,
	}
}

// Chat 进行陪伴对话
func (a *MockCompanionAgent) Chat(ctx context.Context, messages []ai.Message, userEmotion string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	// 根据用户情绪返回不同的回复
	if a.provider != nil {
		if _, isMock := a.provider.(*MockProvider); !isMock {
			content, err := a.provider.Chat(ctx, messages)
			if err != nil {
				return ai.CompanionResponse{}, err
			}
			if strings.TrimSpace(content) != "" {
				emotion := normalizeEmotion(userEmotion)
				return ai.CompanionResponse{
					Content: content,
					Emotion: emotion,
					Action:  actionForEmotion(emotion),
				}, nil
			}
		}
	}

	response := a.getResponseByEmotion(userEmotion)
	return response, nil
}

func normalizeEmotion(userEmotion string) string {
	switch strings.ToLower(strings.TrimSpace(userEmotion)) {
	case "happy", "excited":
		return "happy"
	case "sad", "tired":
		return "encouraging"
	case "frustrated", "confused":
		return "thinking"
	default:
		return "neutral"
	}
}

func actionForEmotion(emotion string) string {
	switch emotion {
	case "happy":
		return "wave"
	case "encouraging":
		return "nod"
	case "thinking":
		return "thinking"
	default:
		return "idle"
	}
}

// GetGreeting 获取问候语
func (a *MockCompanionAgent) GetGreeting(ctx context.Context, profile ai.UserProfile, timeOfDay string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	var greeting string
	var emotion string
	var action string

	switch timeOfDay {
	case "morning":
		greetings := []string{
			"早上好！新的一天开始了，准备好迎接学习的挑战了吗？",
			"早安！一日之计在于晨，让我们从学习开始美好的一天吧！",
			"早上好！今天也要元气满满地学习哦！",
		}
		greeting = greetings[rand.Intn(len(greetings))]
		emotion = "happy"
		action = "wave"
	case "afternoon":
		greetings := []string{
			"下午好！学习了一上午，记得适当休息哦~",
			"午安！下午的学习时光，让我们一起加油！",
			"下午好！保持专注，你一定能取得进步的！",
		}
		greeting = greetings[rand.Intn(len(greetings))]
		emotion = "encouraging"
		action = "nod"
	case "evening":
		greetings := []string{
			"晚上好！今天过得怎么样？来回顾一下今天的学习成果吧！",
			"晚上好！忙碌了一天，现在可以安静地学习了。",
			"晚上好！无论今天如何，明天又是新的开始！",
		}
		greeting = greetings[rand.Intn(len(greetings))]
		emotion = "neutral"
		action = "idle"
	case "night":
		greetings := []string{
			"夜深了，注意休息哦！学习重要，身体更重要~",
			"晚上好！如果累了就早点休息，养精蓄锐明天再战！",
			"深夜学习要注意劳逸结合，别熬太晚哦！",
		}
		greeting = greetings[rand.Intn(len(greetings))]
		emotion = "encouraging"
		action = "nod"
	default:
		greeting = "你好！很高兴见到你，今天想学习什么内容呢？"
		emotion = "happy"
		action = "wave"
	}

	// 根据用户水平添加个性化内容
	if profile.Level == "beginner" {
		greeting += " 作为初学者，不要给自己太大压力，循序渐进就好。"
	} else if profile.Level == "advanced" {
		greeting += " 相信你一定能攻克更高难度的挑战！"
	}

	return ai.CompanionResponse{
		Content: greeting,
		Emotion: emotion,
		Action:  action,
	}, nil
}

// GetEncouragement 获取鼓励语
func (a *MockCompanionAgent) GetEncouragement(ctx context.Context, achievement string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	encouragements := []struct {
		content string
		emotion string
		action  string
	}{
		{
			content: "太棒了！" + achievement + " 你的努力正在开花结果，继续加油！",
			emotion: "happy",
			action:  "celebrate",
		},
		{
			content: "恭喜你完成" + achievement + "！每一步进步都值得庆祝，你比自己想象的更优秀！",
			emotion: "happy",
			action:  "celebrate",
		},
		{
			content: achievement + " 完成得很棒！坚持就是胜利，继续保持这个节奏！",
			emotion: "encouraging",
			action:  "nod",
		},
		{
			content: "看到你的进步我真的很开心！" + achievement + " 证明了你的实力！",
			emotion: "happy",
			action:  "wave",
		},
		{
			content: achievement + " 达成！记住这种成就感，它是你继续前进的动力！",
			emotion: "encouraging",
			action:  "celebrate",
		},
	}

	selected := encouragements[rand.Intn(len(encouragements))]

	return ai.CompanionResponse{
		Content: selected.content,
		Emotion: selected.emotion,
		Action:  selected.action,
	}, nil
}

// getResponseByEmotion 根据情绪获取回复
func (a *MockCompanionAgent) getResponseByEmotion(emotion string) ai.CompanionResponse {
	responses := map[string][]struct {
		content string
		action  string
	}{
		"happy": {
			{"看到你开心我也很高兴！保持这种积极的状态，学习效果会更好哦！", "wave"},
			{"心情好的时候学习效率最高！趁着这股劲头多学一点吧！", "celebrate"},
			{"你的正能量感染到我了！让我们一起愉快地学习吧！", "nod"},
		},
		"sad": {
			{"别难过，学习路上遇到困难很正常。需要我帮你分析一下吗？", "nod"},
			{"抱抱~ 遇到挫折不要灰心，每一次失败都是成长的机会。", "wave"},
			{"心情不好就休息一下，调整好状态再出发。我相信你可以的！", "idle"},
		},
		"frustrated": {
			{"遇到难题了吗？别着急，我们一步一步来解决。需要提示吗？", "thinking"},
			{"学习中的挫败感是暂时的，但收获的知识是永恒的。加油！", "encouraging"},
			{"觉得难说明你在进步！突破舒适区才能成长，我支持你！", "nod"},
		},
		"tired": {
			{"累了就休息一下吧，劳逸结合才能持久。你已经很努力了！", "idle"},
			{"疲惫的时候效率会降低，小憩一会儿再回来学习吧~", "nod"},
			{"身体是革命的本钱，注意休息哦！学习是一场马拉松，不是短跑。", "wave"},
		},
		"excited": {
			{"哇，这么兴奋！是解决了难题还是学到了新知识？分享一下吧！", "celebrate"},
			{"保持这份热情！对知识的渴望是最好的老师！", "wave"},
			{"兴奋的状态太棒了！趁着这股劲多攻克几个知识点吧！", "celebrate"},
		},
		"confused": {
			{"有点迷糊了吗？没关系，哪里不懂我们可以慢慢梳理。", "thinking"},
			{"概念不清楚？我可以帮你解释，或者给你一些学习资料。", "nod"},
			{"困惑是学习的一部分，说明你在思考。让我来帮你理清思路吧！", "thinking"},
		},
	}

	if emotionResponses, ok := responses[emotion]; ok {
		selected := emotionResponses[rand.Intn(len(emotionResponses))]
		return ai.CompanionResponse{
			Content: selected.content,
			Emotion: emotion,
			Action:  selected.action,
		}
	}

	// 默认回复
	return ai.CompanionResponse{
		Content: "我在听呢，继续说下去吧。无论你想聊什么，我都在这里陪着你。",
		Emotion: "neutral",
		Action:  "idle",
	}
}

// init 初始化随机种子
func init() {
	rand.Seed(time.Now().UnixNano())
}
