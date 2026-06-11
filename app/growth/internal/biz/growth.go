package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/sync/errgroup"

	"makejob/pkg/auth"
)

// GrowthRepo data 层必须实现的接口
type GrowthRepo interface {
	GetStudyLogStats(ctx context.Context, userID uint64) (*GrowthSummary, error)
	UpsertStudyLog(ctx context.Context, log *StudyLog) error
	GetWeeklyFocusItems(ctx context.Context, userID uint64) ([]*FocusItem, error)
}

// --- 下游服务客户端接口 ---

// QuestionClient 题目服务客户端接口
type QuestionClient interface {
	GetUserPracticeStats(ctx context.Context, userID uint64) (*PracticeStats, error)
	ListQuestionSets(ctx context.Context, industryCode string) ([]*QuestionSetBrief, error)
}

// PlanClient 计划服务客户端接口
type PlanClient interface {
	GetCurrentPlan(ctx context.Context, userID uint64) (*PlanInfo, error)
}

// LearningArchiveClient 学习档案服务客户端接口
type LearningArchiveClient interface {
	GetWeakTopics(ctx context.Context, userID uint64) ([]string, error)
	GetFocusSignals(ctx context.Context, userID uint64) ([]*FocusSignal, error)
}

// InterviewClient 面试服务客户端接口
type InterviewClient interface {
	GetInterviewStats(ctx context.Context, userID uint64) (*InterviewStats, error)
}

// --- 下游服务响应结构体 ---

// PracticeStats 题目练习统计
type PracticeStats struct {
	TotalDone   int32
	CorrectRate int32
	StreakDays  int32
}

// PlanInfo 当前计划信息
type PlanInfo struct {
	Title          string
	Progress       float64
	CompletedTasks int32
	TotalTasks     int32
}

// FocusSignal 学习焦点信号
type FocusSignal struct {
	Topic  string
	Weight float64
	Source string
}

// InterviewStats 面试统计
type InterviewStats struct {
	TotalInterviews int32
	AvgScore        float64
	LatestScore     float64
}

// QuestionSetBrief 题目集简要信息
type QuestionSetBrief struct {
	ID            uint64
	Title         string
	Description   string
	QuestionCount int32
	Difficulty    string
}

// --- 领域实体 ---

// GrowthSummary 用户成长摘要
type GrowthSummary struct {
	TotalStudyDays  int32
	TotalQuestions  int32
	TotalInterviews int32
	CurrentStreak   int32
	AvgScore        float64
	WeeklyStats     []*WeeklyStat
	WeakTopics      []*TopicWeakness

	// 对齐前端 GrowthSummaryResponse 的扩展字段
	PracticeStats           *GrowthPracticeStats
	StudyDays               int32
	InterviewCount          int32
	CompletedInterviewCount int32
	AverageInterviewScore   float64
	PlanCount               int32
	CurrentPlan             *GrowthCurrentPlan
	FocusSignals            []*GrowthFocusSignal
	TrendSummary            *GrowthTrendSummary
	RecentStudyLogs         []*GrowthStudyLog
	RecentInterviews        []*GrowthInterviewSnapshot
	RecentPlans             []*GrowthPlanSnapshot
}

// GrowthPracticeStats 练习统计详情
type GrowthPracticeStats struct {
	TotalAnswered int32
	CorrectCount  int32
	WrongCount    int32
	AccuracyRate  float64
	TodayCount    int32
	StreakDays    int32
	CategoryStats []*GrowthCategoryStat
}

// GrowthCategoryStat 分类练习统计
type GrowthCategoryStat struct {
	CategoryID   int32
	CategoryName string
	Total        int32
	Correct      int32
	AccuracyRate float64
}

// GrowthCurrentPlan 当前计划卡片
type GrowthCurrentPlan struct {
	ID                     int32
	Title                  string
	Status                 string
	TotalTasks             int32
	CompletedTasks         int32
	Progress               float64
	NextTaskTitle          string
	NextTaskSource         string
	NextTaskReason         string
	NextTaskSourceRef      string
	NextTaskCollectionHint string
}

// GrowthFocusSignal 学习焦点信号（详情版）
type GrowthFocusSignal struct {
	FocusTag                  string
	TopicCode                 string
	TopicTitle                string
	TopicProblemPattern       string
	RelatedQuestionSets       []string
	RecommendedActions        []string
	PrimaryQuestionSet        string
	DominantArchivePhase      string
	DominantArchivePhaseLabel string
	OccurrenceCount           int32
	ArchiveOccurrenceCount    int32
	InterviewOccurrenceCount  int32
	Source                    string
	SourceLabel               string
	Reason                    string
}

// GrowthTrendSummary 趋势摘要
type GrowthTrendSummary struct {
	DominantSource      string
	DominantSourceLabel string
	TopFocusTag         string
	TopTopicCode        string
	TopTopicTitle       string
	Summary             string
}

// GrowthStudyLogSnapshot 最近学习日志快照
type GrowthStudyLog struct {
	ID               int32
	DateKey          string
	Summary          string
	FocusTaskTitle   string
	CompletedCount   int32
	SkippedCount     int32
	CompletedTitles  []string
	SkippedTitles    []string
	LatestActionText string
	UpdatedAt        string
}

// GrowthInterviewSnapshot 最近面试快照
type GrowthInterviewSnapshot struct {
	ID             int32
	Status         string
	Score          int32
	TotalQuestions int32
	CreatedAt      string
	EndedAt        string
}

// GrowthPlanSnapshot 最近计划快照
type GrowthPlanSnapshot struct {
	ID             int32
	Title          string
	Status         string
	TotalTasks     int32
	CompletedTasks int32
	Progress       float64
	StartDate      string
	EndDate        string
}

// WeeklyStat 每周统计
type WeeklyStat struct {
	Week              string
	QuestionsAnswered int32
	InterviewsTaken   int32
	AvgScore          float64
}

// TopicWeakness 知识点薄弱项
type TopicWeakness struct {
	Topic         string
	WeaknessScore float64
	MistakeCount  int32
}

// FocusItem 学习重点项
type FocusItem struct {
	Topic      string
	Source     string
	Weight     float64
	Suggestion string
}

// WeeklyFocusResponse 本周学习重点响应
type WeeklyFocusResponse struct {
	Items   []*FocusItem
	Summary string
	Themes  []*WeeklyFocusTheme
}

// WeeklyFocusTheme 本周补强主题卡片
type WeeklyFocusTheme struct {
	Title                     string
	Reason                    string
	Source                    string
	SourceLabel               string
	FocusTags                 []string
	TopicCodes                []string
	RelatedQuestionSets       []string
	DominantArchivePhase      string
	DominantArchivePhaseLabel string
	OccurrenceCount           int32
	InterviewOccurrenceCount  int32
	Suggestions               []string
}

// StudyLog 学习记录实体
type StudyLog struct {
	ID              uint64
	UserID          uint64
	DateKey         string    // YYYY-MM-DD 格式
	PlanID          uint64
	Summary         string
	Action          string    // practice/interview/study/review
	RefID           uint64
	RefType         string    // question/interview/plan_task
	DurationMinutes int32
	Source          string    // app/web/api
	CreatedAt       time.Time
}

// GrowthUseCase 成长业务用例
type GrowthUseCase struct {
	repo              GrowthRepo
	questionClient    QuestionClient
	planClient        PlanClient
	archiveClient    LearningArchiveClient
	interviewClient   InterviewClient
	logger            log.Logger
}

// NewGrowthUseCase 创建成长用例
func NewGrowthUseCase(
	repo GrowthRepo,
	questionClient QuestionClient,
	planClient PlanClient,
	archiveClient LearningArchiveClient,
	interviewClient InterviewClient,
	logger log.Logger,
) *GrowthUseCase {
	return &GrowthUseCase{
		repo:            repo,
		questionClient:  questionClient,
		planClient:      planClient,
		archiveClient:   archiveClient,
		interviewClient: interviewClient,
		logger:          logger,
	}
}

// GetGrowthSummary 获取用户成长摘要，聚合多个下游服务数据，失败时降级处理
func (uc *GrowthUseCase) GetGrowthSummary(ctx context.Context, userID uint64) (*GrowthSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	var practiceStats *PracticeStats
	var weakTopics []string
	var focusSignals []*FocusSignal
	var interviewStats *InterviewStats
	var studyLogSummary *GrowthSummary
	var planInfo *PlanInfo

	// 并发查询本地学习日志统计
	g.Go(func() error {
		var err error
		studyLogSummary, err = uc.repo.GetStudyLogStats(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取学习日志统计失败: %v", err)
		}
		return nil
	})

	// 并发调用题目服务
	g.Go(func() error {
		var err error
		practiceStats, err = uc.questionClient.GetUserPracticeStats(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取练习统计失败: %v", err)
		}
		return nil // 降级：失败不阻塞
	})

	// 并发调用学习档案 - 弱项主题
	g.Go(func() error {
		var err error
		weakTopics, err = uc.archiveClient.GetWeakTopics(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取弱项主题失败: %v", err)
		}
		return nil
	})

	// 并发调用学习档案 - 焦点信号
	g.Go(func() error {
		var err error
		focusSignals, err = uc.archiveClient.GetFocusSignals(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取焦点信号失败: %v", err)
		}
		return nil
	})

	// 并发调用面试服务
	g.Go(func() error {
		var err error
		interviewStats, err = uc.interviewClient.GetInterviewStats(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取面试统计失败: %v", err)
		}
		return nil
	})

	// 并发调用计划服务 - 当前计划
	g.Go(func() error {
		var err error
		planInfo, err = uc.planClient.GetCurrentPlan(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取当前计划失败: %v", err)
		}
		return nil
	})

	_ = g.Wait() // 所有 goroutine 已做降级处理

	// 组装响应
	summary := &GrowthSummary{
		WeeklyStats:      []*WeeklyStat{},
		WeakTopics:       []*TopicWeakness{},
		FocusSignals:     []*GrowthFocusSignal{},
		RecentStudyLogs:  []*GrowthStudyLog{},
		RecentInterviews: []*GrowthInterviewSnapshot{},
		RecentPlans:      []*GrowthPlanSnapshot{},
	}

	// FIX G2: 从 StudyLog 统计真实学习天数，而非计划任务完成数
	if studyLogSummary != nil {
		summary.TotalStudyDays = studyLogSummary.TotalStudyDays
	}

	if practiceStats != nil {
		summary.TotalQuestions = practiceStats.TotalDone
		summary.CurrentStreak = practiceStats.StreakDays
	}

	if interviewStats != nil {
		summary.TotalInterviews = interviewStats.TotalInterviews
		summary.AvgScore = interviewStats.AvgScore
	}

	// 弱项主题转换
	for _, topic := range weakTopics {
		summary.WeakTopics = append(summary.WeakTopics, &TopicWeakness{
			Topic: topic,
		})
	}

	// 焦点信号补充到弱项
	for _, sig := range focusSignals {
		found := false
		for _, wt := range summary.WeakTopics {
			if wt.Topic == sig.Topic {
				wt.WeaknessScore = sig.Weight
				found = true
				break
			}
		}
		if !found {
			summary.WeakTopics = append(summary.WeakTopics, &TopicWeakness{
				Topic:         sig.Topic,
				WeaknessScore: sig.Weight,
			})
		}
	}

	// --- 对齐前端 GrowthSummaryResponse 的扩展字段填充 ---

	summary.StudyDays = summary.TotalStudyDays
	summary.InterviewCount = summary.TotalInterviews
	summary.CompletedInterviewCount = summary.TotalInterviews
	summary.AverageInterviewScore = summary.AvgScore

	if practiceStats != nil {
		correct := practiceStats.TotalDone * practiceStats.CorrectRate / 100
		wrong := practiceStats.TotalDone - correct
		if wrong < 0 {
			wrong = 0
		}
		summary.PracticeStats = &GrowthPracticeStats{
			TotalAnswered: practiceStats.TotalDone,
			CorrectCount:  correct,
			WrongCount:    wrong,
			AccuracyRate:  float64(practiceStats.CorrectRate),
			StreakDays:    practiceStats.StreakDays,
			CategoryStats: []*GrowthCategoryStat{},
		}
	}

	if planInfo != nil && planInfo.Title != "" {
		summary.PlanCount = 1
		summary.CurrentPlan = &GrowthCurrentPlan{
			Title:          planInfo.Title,
			Progress:       planInfo.Progress * 100,
			CompletedTasks: planInfo.CompletedTasks,
			TotalTasks:     planInfo.TotalTasks,
		}
	}

	// 焦点信号详情转换
	for _, sig := range focusSignals {
		summary.FocusSignals = append(summary.FocusSignals, &GrowthFocusSignal{
			FocusTag:            sig.Topic,
			TopicTitle:          sig.Topic,
			Source:              sig.Source,
			SourceLabel:         focusSourceLabel(sig.Source),
			RelatedQuestionSets: []string{},
			RecommendedActions:  []string{},
			OccurrenceCount:     int32(sig.Weight),
		})
	}

	// 趋势摘要
	if len(focusSignals) > 0 {
		top := focusSignals[0]
		summary.TrendSummary = &GrowthTrendSummary{
			DominantSource:      top.Source,
			DominantSourceLabel: focusSourceLabel(top.Source),
			TopFocusTag:         top.Topic,
			TopTopicTitle:       top.Topic,
			Summary:             buildRecommendationReason(focusSignals, weakTopics, planInfo),
		}
	}

	return summary, nil
}

// focusSourceLabel 将焦点信号来源代码转换为中文标签。
func focusSourceLabel(source string) string {
	switch source {
	case "learning_archive":
		return "最近学习档案"
	case "interview_archive", "interview":
		return "本场面试"
	case "weak_topics":
		return "薄弱知识点"
	case "study_logs":
		return "学习记录"
	default:
		return "成长信号"
	}
}

// GetWeeklyFocus 获取本周学习重点，聚合焦点信号、弱项和计划数据生成推荐
func (uc *GrowthUseCase) GetWeeklyFocus(ctx context.Context, userID uint64) (*WeeklyFocusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	var focusSignals []*FocusSignal
	var weakTopics []string
	var planInfo *PlanInfo

	// 并发获取焦点信号
	g.Go(func() error {
		var err error
		focusSignals, err = uc.archiveClient.GetFocusSignals(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取焦点信号失败: %v", err)
		}
		return nil
	})

	// 并发获取弱项主题（最多 3 个）
	g.Go(func() error {
		var err error
		weakTopics, err = uc.archiveClient.GetWeakTopics(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取弱项主题失败: %v", err)
		}
		return nil
	})

	// 并发获取当前计划
	g.Go(func() error {
		var err error
		planInfo, err = uc.planClient.GetCurrentPlan(gctx, userID)
		if err != nil {
			log.Context(gctx).Warnf("获取当前计划失败: %v", err)
		}
		return nil
	})

	_ = g.Wait()

	// 合并关键词
	keywords := make(map[string]string) // topic -> source
	for _, sig := range focusSignals {
		keywords[sig.Topic] = sig.Source
	}
	for _, topic := range weakTopics {
		if _, exists := keywords[topic]; !exists {
			keywords[topic] = "weak_topics"
		}
	}

	// 生成推荐理由
	reason := buildRecommendationReason(focusSignals, weakTopics, planInfo)

	// 按关键词调用题目集推荐
	items := make([]*FocusItem, 0, len(keywords))
	for topic, source := range keywords {
		items = append(items, &FocusItem{
			Topic:      topic,
			Source:     source,
			Weight:     1.0,
			Suggestion: reason,
		})
	}

	// 尝试获取推荐题目集（FIX G5: 取第一个关键词作为行业代码，不拼接多个）
	if len(keywords) > 0 {
		keywordList := make([]string, 0, len(keywords))
		for k := range keywords {
			keywordList = append(keywordList, k)
		}
		industryCode := keywordList[0]
		sets, err := uc.questionClient.ListQuestionSets(gctx, industryCode)
		if err != nil {
			log.Context(gctx).Warnf("获取推荐题目集失败: %v", err)
		} else {
			for _, s := range sets {
				items = append(items, &FocusItem{
					Topic:      s.Title,
					Source:     "recommended_sets",
					Weight:     float64(s.QuestionCount),
					Suggestion: fmt.Sprintf("推荐练习：%s（%d题）", s.Title, s.QuestionCount),
				})
			}
		}
	}

	return &WeeklyFocusResponse{
		Items:   items,
		Summary: reason,
		Themes:  buildWeeklyFocusThemes(focusSignals, weakTopics, reason),
	}, nil
}

// buildWeeklyFocusThemes 将焦点信号与弱项主题压缩为前端本周补强主题卡片（最多 3 个）。
func buildWeeklyFocusThemes(focusSignals []*FocusSignal, weakTopics []string, reason string) []*WeeklyFocusTheme {
	themes := make([]*WeeklyFocusTheme, 0, 3)
	seen := make(map[string]struct{})

	for _, sig := range focusSignals {
		if len(themes) >= 3 {
			break
		}
		if _, ok := seen[sig.Topic]; ok || sig.Topic == "" {
			continue
		}
		seen[sig.Topic] = struct{}{}
		themes = append(themes, &WeeklyFocusTheme{
			Title:               sig.Topic,
			Reason:              reason,
			Source:              sig.Source,
			SourceLabel:         focusSourceLabel(sig.Source),
			FocusTags:           []string{sig.Topic},
			TopicCodes:          []string{},
			RelatedQuestionSets: []string{},
			OccurrenceCount:     int32(sig.Weight),
			Suggestions:         []string{},
		})
	}

	for _, topic := range weakTopics {
		if len(themes) >= 3 {
			break
		}
		if _, ok := seen[topic]; ok || topic == "" {
			continue
		}
		seen[topic] = struct{}{}
		themes = append(themes, &WeeklyFocusTheme{
			Title:               topic,
			Reason:              reason,
			Source:              "weak_topics",
			SourceLabel:         focusSourceLabel("weak_topics"),
			FocusTags:           []string{topic},
			TopicCodes:          []string{},
			RelatedQuestionSets: []string{},
			Suggestions:         []string{},
		})
	}

	return themes
}

// buildRecommendationReason 根据焦点信号、弱项和计划生成推荐理由
func buildRecommendationReason(focusSignals []*FocusSignal, weakTopics []string, planInfo *PlanInfo) string {
	parts := []string{}

	if len(weakTopics) > 0 {
		parts = append(parts, fmt.Sprintf("薄弱知识点：%s", strings.Join(weakTopics[:min(len(weakTopics), 3)], "、")))
	}

	if len(focusSignals) > 0 {
		topics := make([]string, 0, min(len(focusSignals), 3))
		for _, s := range focusSignals[:min(len(focusSignals), 3)] {
			topics = append(topics, s.Topic)
		}
		parts = append(parts, fmt.Sprintf("重点关注：%s", strings.Join(topics, "、")))
	}

	if planInfo != nil && planInfo.Title != "" {
		parts = append(parts, fmt.Sprintf("当前计划：%s（进度 %.0f%%）", planInfo.Title, planInfo.Progress*100))
	}

	if len(parts) == 0 {
		return "保持当前学习节奏"
	}
	return strings.Join(parts, "；")
}

// SyncStudyLog 同步学习记录，支持 Upsert 语义
func (uc *GrowthUseCase) SyncStudyLog(ctx context.Context, log *StudyLog) (*StudyLog, error) {
	// 从 context 获取用户 ID
	if log.UserID == 0 {
		log.UserID = auth.GetUserIDFromContext(ctx)
	}

	// date_key 为空时使用今天
	if log.DateKey == "" {
		log.DateKey = time.Now().Format("2006-01-02")
	}

	if log.Source == "" {
		log.Source = "app"
	}

	if err := uc.repo.UpsertStudyLog(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
