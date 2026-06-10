I now have all the data needed. Here is the comprehensive, exhaustive field-by-field comparison.

---

## 1. GROWTH SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\growth\v1\growth.proto`
**Frontend file**: `D:\gogogo\makejob\frontend-react\apps\web\src\features\growth\GrowthPage.tsx`

### GrowthSummaryResponse (frontend) vs GrowthSummary (proto)

The frontend expects a rich aggregated response from `GET /growth/summary`. The proto `GrowthSummary` message is dramatically different.

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Proto Field Type | Action Needed |
|---|---|---|---|---|---|
| `practice_stats` | `GrowthPracticeStats \| null` | NO | -- | -- | Add to proto OR transform in handler |
| `study_days` | `number` | YES | `total_study_days` | `int32` | Transform in handler (rename) |
| `interview_count` | `number` | YES | `total_interviews` | `int32` | Transform in handler (rename) |
| `completed_interview_count` | `number` | NO | -- | -- | Add to proto |
| `average_interview_score` | `number` | YES | `avg_score` | `double` | Transform in handler (rename) |
| `plan_count` | `number` | NO | -- | -- | Add to proto |
| `current_plan` | `GrowthCurrentPlan \| null` | NO | -- | -- | Add to proto |
| `focus_signals` | `GrowthFocusSignal[]` | NO | -- | -- | Add to proto |
| `trend_summary` | `GrowthTrendSummary \| null` | NO | -- | -- | Add to proto |
| `recent_study_logs` | `GrowthStudyLog[]` | NO | -- | -- | Add to proto |
| `recent_interviews` | `GrowthInterviewSnapshot[]` | NO | -- | -- | Add to proto |
| `recent_plans` | `GrowthPlanSnapshot[]` | NO | -- | -- | Add to proto |

### GrowthPracticeStats (frontend) -- NO proto equivalent at all

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `total_answered` | `number` | NO | Add to proto |
| `correct_count` | `number` | NO | Add to proto |
| `wrong_count` | `number` | NO | Add to proto |
| `accuracy_rate` | `number` | NO | Add to proto |
| `today_count` | `number` | NO | Add to proto |
| `streak_days` | `number` | NO | Add to proto |
| `category_stats` | `GrowthCategoryStat[]` | NO | Add to proto |

### GrowthCategoryStat (frontend) -- NO proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `category_id` | `number` | NO | Add to proto |
| `category_name` | `string` | NO | Add to proto |
| `total` | `number` | NO | Add to proto |
| `correct` | `number` | NO | Add to proto |
| `accuracy_rate` | `number` | NO | Add to proto |

### GrowthCurrentPlan (frontend) -- NO proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `id` | `number` | NO | Add to proto |
| `title` | `string` | NO | Add to proto |
| `status` | `string` | NO | Add to proto |
| `total_tasks` | `number` | NO | Add to proto |
| `completed_tasks` | `number` | NO | Add to proto |
| `progress` | `number` | NO | Add to proto |
| `next_task_title` | `string` | NO | Add to proto |
| `next_task_source` | `string` | NO | Add to proto |
| `next_task_reason` | `string` | NO | Add to proto |
| `next_task_source_ref` | `string` | NO | Add to proto |
| `next_task_collection_hint` | `string` | NO | Add to proto |

### GrowthFocusSignal (frontend) -- NO proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `focus_tag` | `string` | NO | Add to proto |
| `topic_code` | `string` | NO | Add to proto |
| `topic_title` | `string` | NO | Add to proto |
| `topic_problem_pattern` | `string` | NO | Add to proto |
| `related_question_sets` | `string[]` | NO | Add to proto |
| `recommended_actions` | `string[]` | NO | Add to proto |
| `primary_question_set` | `string` | NO | Add to proto |
| `dominant_archive_phase` | `string` | NO | Add to proto |
| `dominant_archive_phase_label` | `string` | NO | Add to proto |
| `occurrence_count` | `number` | NO | Add to proto |
| `archive_occurrence_count` | `number` | NO | Add to proto |
| `interview_occurrence_count` | `number` | NO | Add to proto |
| `source` | `string` | NO | Add to proto |
| `source_label` | `string` | NO | Add to proto |
| `reason` | `string` | NO | Add to proto |

### GrowthTrendSummary (frontend) -- NO proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `dominant_source` | `string` | NO | Add to proto |
| `dominant_source_label` | `string` | NO | Add to proto |
| `top_focus_tag` | `string` | NO | Add to proto |
| `top_topic_code` | `string` | NO | Add to proto |
| `top_topic_title` | `string` | NO | Add to proto |
| `summary` | `string` | NO | Add to proto |

### GrowthStudyLog (frontend) vs StudyLog (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `date_key` | `string` | NO | -- | Add to proto |
| `summary` | `string` | NO | -- | Add to proto |
| `focus_task_title` | `string` | NO | -- | Add to proto |
| `completed_count` | `number` | NO | -- | Add to proto |
| `skipped_count` | `number` | NO | -- | Add to proto |
| `completed_titles` | `string[]` | NO | -- | Add to proto |
| `skipped_titles` | `string[]` | NO | -- | Add to proto |
| `latest_action_text` | `string` | NO | -- | Add to proto |
| `updated_at` | `string` | NO | `created_at` exists | Transform in handler |

### GrowthInterviewSnapshot (frontend) -- NO proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `id` | `number` | NO | Add to proto |
| `status` | `string` | NO | Add to proto |
| `score` | `number` | NO | Add to proto |
| `total_questions` | `number` | NO | Add to proto |
| `created_at` | `string` | NO | Add to proto |
| `ended_at` | `string` | NO | Add to proto |

### GrowthPlanSnapshot (frontend) -- NO proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `id` | `number` | NO | Add to proto |
| `title` | `string` | NO | Add to proto |
| `status` | `string` | NO | Add to proto |
| `total_tasks` | `number` | NO | Add to proto |
| `completed_tasks` | `number` | NO | Add to proto |
| `progress` | `number` | NO | Add to proto |
| `start_date` | `string` | NO | Add to proto |
| `end_date` | `string` | NO | Add to proto |

### WeeklyFocus (proto) vs WeeklyFocusResponse (frontend, in weeklyFocus.ts)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `themes` | `WeeklyFocusTheme[]` | PARTIAL | `items` (repeated FocusItem) | Transform in handler; names differ |

### WeeklyFocusTheme (frontend) vs FocusItem (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `title` | `string` | NO | -- | Add to proto |
| `reason` | `string` | NO | -- | Add to proto |
| `source` | `string` | YES | `source` | Match |
| `source_label` | `string` | NO | -- | Add to proto |
| `focus_tags` | `string[]` | NO | -- | Add to proto |
| `topic_codes` | `string[]` | NO | -- | Add to proto |
| `related_question_sets` | `string[]` | NO | -- | Add to proto |
| `dominant_archive_phase` | `string` | NO | -- | Add to proto |
| `dominant_archive_phase_label` | `string` | NO | -- | Add to proto |
| `occurrence_count` | `number` | NO | -- | Add to proto |
| `interview_occurrence_count` | `number` | NO | -- | Add to proto |
| `suggestions` | `string[]` | NO | -- | Add to proto |

---

## 2. COMPANION SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\companion\v1\companion.proto`
**Frontend file**: `D:\gogogo\makejob\frontend-react\apps\web\src\features\companion\companionTypes.ts`

### CompanionChatResponse (proto) vs CompanionChatReply (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Proto Field Type | Action Needed |
|---|---|---|---|---|---|
| `content` | `string` | NO | -- | -- | Add to proto (frontend checks both `content` and `reply`) |
| `reply` | `string` | YES | `reply` | `string` | Match |
| `emotion` | `string` | YES | `emotion` | `string` | Match |
| `mood` | `string` | NO | -- | -- | Add to proto |
| `action` | `string` | NO | -- | -- | Add to proto |
| `audio_url` | `string` | NO | -- | -- | Add to proto |
| `audio_duration` | `number` | NO | -- | -- | Add to proto |
| `audio_format` | `string` | NO | -- | -- | Add to proto |
| `audio_sample_rate` | `number` | NO | -- | -- | Add to proto |
| `live2d_directive` | `Live2DDirective \| null` | NO | -- | -- | Add to proto |
| `suggestions` (proto) | -- | YES (in proto) | `suggestions` | `repeated string` | Not consumed by frontend ChatReply |

### CompanionState (proto) -- consumed indirectly

The proto `CompanionState` has `emotion`, `last_topic`, `last_active_at`. The frontend does not directly consume this message type via a typed interface; it uses the chat reply instead.

### SynthesizeSpeechResponse (proto) -- consumed indirectly

The proto has `audio_data` (bytes) and `audio_url` (string). The frontend `CompanionChatReply` has `audio_url` but not `audio_data`.

### CompanionPlanDetail (frontend) vs PlanDetail (plan.proto)

The frontend companion module re-uses the Plan service types. The `CompanionPlanDetail` interface extends beyond what `plan.proto`'s `PlanDetail` provides:

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `industry_id` | `number` | NO | -- | Add to proto |
| `industry_code` | `string` | NO | -- | Add to proto |
| `title` | `string` | YES | `title` | Match |
| `description` | `string` | YES | `description` | Match |
| `phase` | `string` | NO | -- | Add to proto |
| `phase_goal` | `string` | NO | -- | Add to proto |
| `entry_phase` | `string` | NO | -- | Add to proto |
| `adjustment_summaries` | `string[]` | NO | -- | Add to proto |
| `adjustment_reason_codes` | `string[]` | NO | -- | Add to proto |
| `phase_blueprint_summary` | `PhaseBlueprintSummaryEntry[]` | NO | -- | Add to proto |
| `status` | `string` | YES | `status` | Match |
| `async_task_id` | `number` | NO | -- | Add to proto |
| `task_status` | `string` | NO | -- | Add to proto |
| `task_error` | `string` | NO | -- | Add to proto |
| `total_tasks` | `number` | YES | `total_tasks` | Match |
| `completed_tasks` | `number` | YES | `completed_tasks` | Match |
| `progress` | `number` | YES | `progress` | Match |
| `start_date` | `string` | NO | -- | Add to proto |
| `end_date` | `string` | NO | -- | Add to proto |
| `tasks` | `CompanionPlanTask[]` | YES | `tasks` (repeated TaskDetail) | Match (but task sub-fields differ) |
| `created_at` | `string` | YES | `created_at` | Match |

### CompanionPlanTask (frontend) vs TaskDetail (plan.proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `title` | `string` | YES | `title` | Match |
| `description` | `string` | YES | `description` | Match |
| `task_type` | `string` | YES | `task_type` | Match |
| `phase` | `string` | YES | `phase` | Match |
| `phase_goal` | `string` | NO | -- | Add to proto |
| `status` | `string` | YES | `status` | Match |
| `due_date` | `string` | NO | -- | Add to proto |
| `completed_at` | `string` | YES | `completed_at` | Match |
| `day_number` | `number` | YES | `day_number` | Match |
| `sort_order` | `number` | NO | `order_index` exists | Transform in handler (rename) |
| `source` | `string` | NO | -- | Add to proto |
| `source_label` | `string` | NO | -- | Add to proto |
| `reason` | `string` | NO | -- | Add to proto |
| `priority_explanation` | `string` | NO | -- | Add to proto |
| `source_ref` | `string` | NO | -- | Add to proto |
| `collection_hint` | `string` | NO | -- | Add to proto |

### CompanionPlanProgress (frontend) vs PlanProgressResponse (plan.proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `plan_id` | `number` | YES | `plan_id` | Match |
| `total_tasks` | `number` | YES | `total_tasks` | Match |
| `completed_tasks` | `number` | YES | `completed_tasks` | Match |
| `skipped_tasks` | `number` | YES | `skipped_tasks` | Match |
| `in_progress_tasks` | `number` | YES | `in_progress_tasks` | Match |
| `pending_tasks` | `number` | YES | `pending_tasks` | Match |
| `progress` | `number` | YES | `progress` | Match |

### CompanionGeneratePlanPayload (frontend) vs CreatePlanRequest (plan.proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `level` | `string` | YES | `level` | Match |
| `daily_study_time` | `number` | YES | `daily_study_minutes` | Transform in handler (rename) |
| `weak_topics` | `string[]` | YES | `weak_topics` | Match |
| `goal_description` | `string` | YES | `goal_description` | Match |
| `duration_days` | `number` | YES | `duration_days` | Match |
| `industry_id` | `number` | NO | -- | Add to proto |
| `industry_code` | `string` | NO | `industry` exists | Transform in handler (rename) |

### CompanionStudyLogPayload (frontend) vs SyncStudyLogRequest (growth.proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `date_key` | `string` | NO | -- | Add to proto |
| `plan_id` | `number` | NO | -- | Add to proto |
| `summary` | `string` | NO | -- | Add to proto |
| `focus_task_title` | `string` | NO | -- | Add to proto |
| `completed_count` | `number` | NO | -- | Add to proto |
| `skipped_count` | `number` | NO | -- | Add to proto |
| `completed_titles` | `string[]` | NO | -- | Add to proto |
| `skipped_titles` | `string[]` | NO | -- | Add to proto |
| `latest_action_text` | `string` | NO | -- | Add to proto |
| `user_id` (proto) | -- | YES | `user_id` | Frontend does not send; handler injects |
| `action` (proto) | -- | YES | `action` | Frontend does not send; handler injects |
| `ref_id` (proto) | -- | YES | `ref_id` | Frontend does not send; handler injects |
| `duration_seconds` (proto) | -- | YES | `duration_seconds` | Frontend does not send; handler injects |

### CompanionSelectableLive2DModel (frontend) vs SelectableLive2DModel (admin.proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `key` | `string` | YES | `key` | Match |
| `name` | `string` | YES | `name` | Match |
| `scene` | `string` | YES | `scene` | Match |
| `model_url` | `string` | YES | `model_url` | Match |
| `thumbnail_url` | `string` | YES | `thumbnail_url` | Match |
| `config_json` | `string` | YES | `config_json` | Match |
| `source` | `string` | YES | `source` | Match |
| `match_type` | `string` | YES | `match_type` | Match |
| `is_generic` | `boolean` | YES | `is_generic` | Match |
| `is_recommended` | `boolean` | YES | `is_recommended` | Match |
| `motions` | `Array<{key,group,file,label}>` | YES | `repeated Live2DMotionInfo motions` | Match |

---

## 3. PLAN SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\plan\v1\plan.proto`
**Frontend**: Used indirectly through companion. The proto is reasonably aligned with what the companion module consumes, but many frontend fields (listed in companion section above) are absent from the proto.

### PlanDetail (proto) -- fields the frontend expects but proto lacks

| Missing Frontend Field | Type | Action |
|---|---|---|
| `industry_id` | `number` | Add to proto |
| `industry_code` | `string` | Add to proto |
| `phase` | `string` | Add to proto |
| `phase_goal` | `string` | Add to proto |
| `entry_phase` | `string` | Add to proto |
| `adjustment_summaries` | `string[]` | Add to proto |
| `adjustment_reason_codes` | `string[]` | Add to proto |
| `phase_blueprint_summary` | `array` | Add to proto |
| `async_task_id` | `number` | Add to proto |
| `task_status` | `string` | Add to proto |
| `task_error` | `string` | Add to proto |
| `start_date` | `string` | Add to proto |
| `end_date` | `string` | Add to proto |

### TaskDetail (proto) -- fields the frontend expects but proto lacks

| Missing Frontend Field | Type | Action |
|---|---|---|
| `phase_goal` | `string` | Add to proto |
| `due_date` | `string` | Add to proto |
| `source` | `string` | Add to proto |
| `source_label` | `string` | Add to proto |
| `reason` | `string` | Add to proto |
| `priority_explanation` | `string` | Add to proto |
| `source_ref` | `string` | Add to proto |
| `collection_hint` | `string` | Add to proto |

Also: proto has `order_index` but frontend expects `sort_order` -- needs rename in handler.

### ListPlansRequest (proto) -- frontend sends different field names

| Frontend Field | Proto Field | Action |
|---|---|---|
| `page` | `page` | Match |
| `page_size` | `page_size` | Match |
| `user_id` | `user_id` | Handler injects from auth |

### CreatePlanRequest (proto) -- field name mismatches

| Frontend Field | Proto Field | Action |
|---|---|---|
| `daily_study_time` | `daily_study_minutes` | Transform in handler |
| `industry_code` | `industry` | Transform in handler |
| `industry_id` | -- | Add to proto |

---

## 4. INTERVIEW SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\interview\v1\interview.proto`
**Frontend file**: `D:\gogogo\makejob\frontend-react\apps\web\src\features\interview\interviewTypes.ts`

### InterviewResponse (proto) vs InterviewCreateResponse (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `interview_id` | `number` | YES | `id` | Transform in handler (rename) |
| `status` | `string` | YES | `status` | Match |
| `async_task_id` | `number` | NO | -- | Add to proto |
| `task_status` | `string` | NO | -- | Add to proto |
| `task_error` | `string` | NO | -- | Add to proto |
| `first_question` | `InterviewQuestion \| null` | YES | `first_question` | Match |
| `created_at` | `string` | YES | `created_at` | Match |

### InterviewHistoryItem (frontend) vs InterviewResponse (proto, via ListInterviewsResponse)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `status` | `string` | YES | `status` | Match |
| `score` | `number` | NO | -- | Add to proto |
| `total_questions` | `number` | NO | -- | Add to proto |
| `started_at` | `string` | NO | -- | Add to proto |
| `ended_at` | `string` | NO | -- | Add to proto |
| `created_at` | `string` | YES | `created_at` | Match |

### InterviewDetailResponse (frontend) vs InterviewDetail (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `industry_code` | `string` | YES | `industry_code` | Match |
| `status` | `string` | YES | `status` | Match |
| `async_task_id` | `number` | NO | -- | Add to proto |
| `task_status` | `string` | NO | -- | Add to proto |
| `task_error` | `string` | NO | -- | Add to proto |
| `score` | `number` | NO | -- | Add to proto |
| `total_questions` | `number` | NO | -- | Add to proto |
| `messages` | `InterviewMessage[]` | YES | `messages` | Match |
| `current_question` | `InterviewQuestion \| null` | NO | -- | Add to proto |
| `started_at` | `string` | NO | -- | Add to proto |
| `ended_at` | `string` | NO | -- | Add to proto |
| `user_id` (proto) | -- | YES | `user_id` | Not in frontend type |
| `interview_mode` (proto) | -- | YES | `interview_mode` | Not in frontend type |
| `report` (proto) | -- | YES | `report` | Not in frontend type |

### InterviewMessage (frontend) vs InterviewMessage (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `role` | `string` | YES | `role` | Match |
| `content` | `string` | YES | `content` | Match |
| `message_type` | `string` | YES | `message_type` | Match |
| `question` | `InterviewQuestion \| null` | NO | -- | Add to proto |
| `created_at` | `string` | YES | `created_at` | Match |
| `id` (proto) | -- | YES | `id` | Not in frontend type |

### InterviewQuestion (frontend) vs InterviewQuestion (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `question` | `string` | YES | `question` | Match |
| `topic` | `string` | YES | `topic` | Match |
| `difficulty` | `string` | YES | `difficulty` | Match |
| `type` | `string` | YES | `type` | Match |
| `hints` | `string` | YES | `hints` | Match |
| `language` | `string` | YES | `language` | Match |
| `starter_code` | `string` | YES | `starter_code` | Match |
| `editor_mode` | `string` | YES | `editor_mode` | Match |
| `evaluation_mode` | `string` | YES | `evaluation_mode` | Match |
| `live2d_directive` | `Live2DDirective \| null` | YES | `live2d_directive` | Match |

### InterviewFeedback (frontend) vs AnswerFeedback (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `score` | `number` | YES | `score` | Match |
| `is_correct` | `boolean` | YES | `is_correct` | Match |
| `feedback` | `string` | YES | `feedback` | Match |
| `key_points` | `string[]` | YES | `key_points` | Match |
| `suggestions` | `string` | YES | `suggestions` | Match |
| `follow_up` | `string` | YES | `follow_up` | Match |

### InterviewAnswerResponse (frontend) -- NO direct proto equivalent

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `feedback` | `InterviewFeedback \| null` | NO | Handler composes from AnswerFeedback |
| `next_question` | `InterviewQuestion \| null` | NO | Handler composes from AnswerFeedback.next_question |
| `is_finished` | `boolean` | NO | Add to proto or derive |

### InterviewNextQuestionResponse (frontend) vs NextQuestionResponse (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `question` | `InterviewQuestion \| null` | YES | `question` | Match |
| `question_no` | `number` | NO | -- | Add to proto |
| `is_last` | `boolean` | YES | `is_last` | Match |

### InterviewReport (frontend) vs InterviewReport (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `overall_score` | `number` | YES | `overall_score` | Match |
| `total_questions` | `number` | YES | `total_questions` | Match |
| `correct_count` | `number` | YES | `correct_count` | Match |
| `dimension_scores` | `Record<string, number>` | YES | `map<string, double>` | Match |
| `strengths` | `string[]` | YES | `strengths` | Match |
| `weaknesses` | `string[]` | YES | `weaknesses` | Match |
| `suggestions` | `string[]` | YES | `suggestions` | Match |
| `summary` | `string` | YES | `summary` | Match |
| `coding_diagnostics` | `InterviewCodingDiagnosis[]` | YES | `coding_diagnostics` | Match |

### InterviewCodingDiagnosis (frontend) vs CodingDiagnosis (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `question_index` | `number` | YES | `question_index` | Match |
| `language` | `string` | YES | `language` | Match |
| `score` | `number` | YES | `score` | Match |
| `mistake_tags` | `string[]` | YES | `mistake_tags` | Match |
| `strength_tags` | `string[]` | YES | `strength_tags` | Match |
| `evidence` | `string[]` | NO | `evidence_summary` (string) | Transform in handler (rename + type mismatch: proto string vs frontend string[]) |
| `suggestions` | `string[]` | YES | `suggestions` | Match |
| `process_summary` | `string` | NO | -- | Add to proto |

### InterviewReportResponse (frontend) -- wraps InterviewReport

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `interview_id` | `number` | YES (proto InterviewReport) | Match |
| `status` | `string` | YES (proto InterviewReport) | Match |
| `async_task_id` | `number` | NO | Add to proto |
| `task_status` | `string` | NO | Add to proto |
| `task_error` | `string` | NO | Add to proto |
| `report` | `InterviewReport \| null` | NO | Handler wraps the proto InterviewReport |
| `duration_seconds` | `number` | NO | Add to proto |
| `completed_at` | `string` | NO | Add to proto |

---

## 5. QUESTION SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\question\v1\question.proto`
**Frontend files**: `D:\gogogo\makejob\frontend-react\apps\web\src\shared\practiceCatalog.ts`, `D:\gogogo\makejob\frontend-react\apps\web\src\features\practice\PracticeDetailPages.tsx`

### PracticeQuestion (frontend) vs QuestionSummary (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `title` | `string` | YES | `title` | Match |
| `difficulty` | `string` | YES | `difficulty` | Match |
| `type` | `string` | YES | `type` | Match |
| `category_id` | `number` | NO | -- | Add to proto |
| `industry_id` | `number` | NO | -- | Add to proto |
| `category_name` | `string` | YES | `category_name` | Match |
| `pass_rate` | `number` | NO | -- | Add to proto |
| `is_favorite` | `boolean` | NO | -- | Add to proto |
| `tags` | `string` | NO | -- | Add to proto |
| `industry_code` (proto) | -- | YES | `industry_code` | Not in frontend type |

### PracticeQuestionDetail (frontend) vs QuestionDetail (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| extends PracticeQuestion | -- | -- | -- | (see above) |
| `content` | `string` | YES | `content` | Match |
| `options_json` | `string` | NO | -- | Add to proto |
| `answer` | `string` | YES | `reference_answer` | Transform in handler (rename) |
| `explanation` | `string` | YES | `explanation` | Match |
| `tag_list` | `string[]` | YES | `repeated string tags` | Transform in handler (rename) |
| `solution` | `QuestionSolution \| null` | NO | -- | Add to proto |
| `judge_config` | `QuestionJudgeConfig \| null` | NO | -- | Add to proto |
| `answer_template` | `QuestionAnswerTemplate \| null` | NO | -- | Add to proto |
| `is_favorited` | `boolean` | YES | `is_favorited` | Match |
| `user_note` | `PracticeNote \| null` | NO | -- | Add to proto or fetch separately |
| `starter_code` (proto) | -- | YES | `starter_code` | Not in top-level frontend type (inside judge_config) |
| `language` (proto) | -- | YES | `language` | Not in frontend type |
| `evaluation_mode` (proto) | -- | YES | `evaluation_mode` | Not in top-level frontend type |
| `test_cases` (proto) | -- | YES | `test_cases` | Not in frontend type (inside judge_config) |
| `created_at` (proto) | -- | YES | `created_at` | Not in frontend type |

### QuestionSolution (frontend) -- NO proto equivalent

| Frontend Field | Type | Action |
|---|---|---|
| `summary` | `string` | Add to proto |
| `approach` | `string` | Add to proto |
| `key_steps` | `string[]` | Add to proto |
| `edge_cases` | `string[]` | Add to proto |
| `complexity` | `string` | Add to proto |
| `common_mistakes` | `string[]` | Add to proto |
| `recommended_tags` | `string[]` | Add to proto |

### QuestionAnswerTemplate (frontend) -- NO proto equivalent

| Frontend Field | Type | Action |
|---|---|---|
| `core_conclusion` | `string` | Add to proto |
| `key_points` | `string[]` | Add to proto |
| `sample_answer` | `string` | Add to proto |
| `follow_ups` | `string[]` | Add to proto |
| `pitfalls` | `string[]` | Add to proto |

### QuestionJudgeConfig (frontend) -- NO proto equivalent

| Frontend Field | Type | Action |
|---|---|---|
| `evaluation_mode` | `string` | Add to proto |
| `default_language` | `string` | Add to proto |
| `allowed_languages` | `string[]` | Add to proto |
| `starter_code` | `string` | Add to proto |
| `public_test_cases` | `TestCase[]` | Add to proto |
| `time_limit_ms` | `number` | Add to proto |
| `memory_limit_mb` | `number` | Add to proto |

### SubmitAnswerResponse (proto) vs SubmitAnswerResult (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `is_correct` | `boolean` | YES | `is_correct` | Match |
| `correct_answer` | `string` | YES | `correct_answer` | Match |
| `explanation` | `string` | YES | `explanation` | Match |
| `ai_analysis` | `string` | NO | -- | Add to proto |
| `evaluation_mode` | `string` | NO | -- | Add to proto |
| `judge_summary` | `JudgeSummary \| null` | NO | -- | Add to proto |
| `score` (proto) | -- | YES | `score` | Not in frontend type |
| `feedback` (proto) | -- | YES | `feedback` | Not in frontend type |
| `key_points` (proto) | -- | YES | `key_points` | Not in frontend type |

### PracticeStats (frontend) vs UserPracticeStats (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `total_answered` | `number` | YES | `total_answered` | Match |
| `correct_count` | `number` | YES | `total_correct` | Transform in handler (rename) |
| `wrong_count` | `number` | NO | -- | Add to proto or derive |
| `accuracy_rate` | `number` | YES | `accuracy` | Transform in handler (rename) |
| `today_count` | `number` | NO | -- | Add to proto |
| `streak_days` | `number` | YES | `streak_days` | Match |

### PracticeQuestionSetSummary (frontend) vs ListQuestionSetsResponse/QuestionSetSummary (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `slug` | `string` | NO | -- | Add to proto |
| `title` | `string` | YES | `title` | Match |
| `description` | `string` | YES | `description` | Match |
| `focus_tags` | `string[]` | NO | -- | Add to proto |
| `question_count` | `number` | YES | `question_count` | Match |
| `questions` | `PracticeQuestionSetPreview[]` | NO | -- | Add to proto |

### PracticeRecommendationItem (frontend, in practiceRecommendations.ts) -- NO proto equivalent at all

The proto `PracticeRecommendationResponse` has `repeated RecommendedQuestion questions` and `string reason`. The frontend expects a much richer structure:

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `question` | `PracticeRecommendationQuestion` | PARTIAL | `question_id`, `title`, `difficulty` | Transform in handler |
| `focus_tag` | `string` | NO | -- | Add to proto |
| `topic_code` | `string` | NO | -- | Add to proto |
| `topic_title` | `string` | NO | -- | Add to proto |
| `topic_problem_pattern` | `string` | NO | -- | Add to proto |
| `related_question_sets` | `string[]` | NO | -- | Add to proto |
| `recommended_actions` | `string[]` | NO | -- | Add to proto |
| `primary_question_set` | `string` | NO | -- | Add to proto |
| `dominant_archive_phase` | `string` | NO | -- | Add to proto |
| `dominant_archive_phase_label` | `string` | NO | -- | Add to proto |
| `recommendation_mode` | `string` | NO | -- | Add to proto |
| `reason` | `string` | YES | `reason` (on response, not per-item) | Transform in handler |
| `source_type` | `string` | NO | -- | Add to proto |
| `priority` | `number` | NO | -- | Add to proto |
| `occurrence_count` | `number` | NO | -- | Add to proto |
| `priority_explanation` | `string` | NO | -- | Add to proto |

### GeneratedExamResponse (frontend) vs ExamResponse (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `exam_id` | `string` | NO | -- | Add to proto |
| `time_limit` | `number` | YES | `time_limit_minutes` | Transform in handler (rename) |
| `questions` | `GeneratedExamQuestion[]` | YES | `repeated QuestionDetail` | Transform in handler |

### MistakeTopicCard (frontend, in mistakeTopics.ts) vs MistakeTopicCard (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `code` | `string` | YES | `code` | Match |
| `tag` | `string` | YES | `tag` | Match |
| `title` | `string` | YES | `title` | Match |
| `problem_pattern` | `string` | YES | `problem_pattern` | Match |
| `root_causes` | `string[]` | YES | `root_causes` | Match |
| `self_check_list` | `string[]` | YES | `self_check_list` | Match |
| `practice_directions` | `string[]` | YES | `practice_directions` | Match |
| `recommended_actions` | `string[]` | YES | `recommended_actions` | Match |
| `related_question_sets` | `string[]` | YES | `related_question_sets` | Match |

---

## 6. COMMUNITY SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\community\v1\community.proto`
**Frontend file**: `D:\gogogo\makejob\frontend-react\apps\web\src\features\community\CommunityPages.tsx`

### CommunityPostItem (frontend) vs PostSummary/PostDetail (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Proto Source | Action Needed |
|---|---|---|---|---|---|
| `id` | `number` | YES | `id` | Both | Match |
| `post_type` | `string` | NO | -- | -- | Add to proto (PostSummary) |
| `title` | `string` | YES | `title` | Both | Match |
| `content` | `string` | YES | `content` | PostDetail only | Match |
| `summary` | `string` | NO | -- | -- | Add to proto |
| `tags` | `string[]` | NO | -- | -- | Add to proto |
| `view_count` | `number` | YES | `view_count` | PostDetail only | Add to PostSummary |
| `comment_count` | `number` | YES | `comment_count` | Both | Match |
| `like_count` | `number` | YES | `like_count` | Both | Match |
| `is_pinned` | `boolean` | NO | -- | -- | Add to proto |
| `is_recommended` | `boolean` | NO | -- | -- | Add to proto |
| `created_at` | `string` | YES | `created_at` | Both | Match |
| `updated_at` | `string` | NO | -- | -- | Add to proto |
| `is_liked` | `boolean` | YES | `is_liked` | PostDetail only | Add to PostSummary |
| `is_author` | `boolean` | NO | -- | -- | Add to proto |
| `author` | `CommunityPostAuthor` | PARTIAL | `author_id` + `author_name` | Both | Transform in handler (compose object) |
| `author_avatar` (proto) | -- | YES | `author_avatar` | PostDetail only | Not in frontend type |
| `category` (proto) | -- | YES | `category` | Both | Not in frontend type |

### CommunityCommentItem (frontend) vs Comment (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `content` | `string` | YES | `content` | Match |
| `created_at` | `string` | YES | `created_at` | Match |
| `updated_at` | `string` | NO | -- | Add to proto |
| `is_author` | `boolean` | NO | -- | Add to proto |
| `author` | `CommunityPostAuthor` | PARTIAL | `author_id` + `author_name` | Transform in handler |
| `post_id` (proto) | -- | YES | `post_id` | Not in frontend type |
| `author_id` (proto) | -- | YES | `author_id` | Composed into author object |

### CommunityLikeToggleResponse (frontend) vs LikeResponse (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `liked` | `boolean` | YES | `liked` | Match |
| `like_count` | `number` | YES | `like_count` | Match |

### CommunityPostPayload (frontend) vs CreatePostRequest (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `post_type` | `string` | YES | `post_type` | Match |
| `title` | `string` | YES | `title` | Match |
| `content` | `string` | YES | `content` | Match |
| `tags` | `string[]` | YES | `tags` (string, comma-separated) | Transform in handler (array to CSV) |
| `author_id` (proto) | -- | YES | `author_id` | Handler injects from auth |

### PageResult (frontend) vs ListPostsResponse (proto)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `list` | `T[]` | YES | `posts` | Transform in handler (rename) |
| `total` | `number` | NO (uses `page_result`) | -- | Transform from page_result |
| `page` | `number` | NO (uses `page_result`) | -- | Transform from page_result |
| `page_size` | `number` | NO (uses `page_result`) | -- | Transform from page_result |

---

## 7. ADMIN SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\admin\v1\admin.proto`
**Frontend files**: `D:\gogogo\makejob\frontend-react\apps\admin\src\features\*`

### DashboardResponse (proto) vs DashboardStats (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `total_users` | `number` | YES | `total_users` | Match |
| `total_questions` | `number` | YES | `total_questions` | Match |
| `total_interviews` | `number` | YES | `total_interviews` | Match |
| `today_active_users` | `number` | YES | `today_active_users` | Match |
| `pro_members` | `number` | YES | `pro_members` | Match |
| `new_users_today` | `number` | YES | `new_users_today` | Match |

### QuestionInfo (proto) vs QuestionListItem (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `category_id` | `number` | YES | `category_id` | Match |
| `category_name` | `string` | YES | `category_name` | Match |
| `industry_id` | `number` | YES | `industry_id` | Match |
| `type` | `string` | YES | `type` | Match |
| `difficulty` | `string` | YES | `difficulty` | Match |
| `title` | `string` | YES | `title` | Match |
| `content` | `string` | YES | `content` | Match |
| `options` | `string[]` | NO | `options_json` (string) | Transform in handler (JSON string to array) |
| `answer` | `string` | YES | `answer` | Match |
| `explanation` | `string` | YES | `explanation` | Match |
| `solution` | `QuestionSolution \| null` | NO | `solution_json` (string) | Transform in handler (JSON string to object) |
| `judge_config` | `QuestionJudgeConfig \| null` | NO | `judge_config_json` (string) | Transform in handler (JSON string to object) |
| `answer_template` | `QuestionAnswerTemplate \| null` | NO | `answer_template_json` (string) | Transform in handler (JSON string to object) |
| `tags` | `string[]` | NO | `tags` (string, CSV) | Transform in handler (CSV to array) |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `created_at` | `string` | YES | `created_at` | Match |
| `updated_at` | `string` | YES | `updated_at` | Match |
| `industry_name` (proto) | -- | YES | `industry_name` | Not in frontend type |

### AdminConfigItem (proto) vs AdminConfigItem (frontend - AIConfigPage)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | NO | -- | Add to proto |
| `config_key` | `string` | YES | `key` | Transform in handler (rename) |
| `config_value` | `string` | YES | `value` | Transform in handler (rename) |
| `config_type` | `string` | YES | `config_type` | Match |
| `description` | `string` | YES | `description` | Match |

### GetAIConfigsResponse (proto) vs AIConfigResponse (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `configs` | `Record<string, string>` | YES | `configs` | Match |
| `items` | `AdminConfigItem[]` | YES | `items` | Match (but sub-field names differ) |
| `support` | `object` | NO | -- | Add to proto |
| `warnings` | `string[]` | NO | -- | Add to proto |
| `presets` | `AIPresetSummary[]` | YES | `presets` | Match (but sub-structure differs) |
| `active_preset_id` | `number \| null` | YES | `active_preset_id` | Match |

### AIPreset (proto) vs AIPresetSummary (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `name` | `string` | YES | `name` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `updated_at` | `string` | YES | `updated_at` | Match |
| `configs` | `Record<string, string>` | YES | `configs` | Match |
| `provider` (proto) | -- | YES | `provider` | Not in frontend type |
| `model` (proto) | -- | YES | `model` | Not in frontend type |
| `params` (proto) | -- | YES | `params` | Not in frontend type |
| `is_default` (proto) | -- | YES | `is_default` | Not in frontend type |

### AICallLog (proto) vs AICallLogItem (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `created_at` | `string` | YES | `created_at` | Match |
| `trace_id` | `string` | NO | -- | Add to proto (only in detail) |
| `task_id` | `number` | NO | -- | Add to proto |
| `source` | `string` | NO | -- | Add to proto (only in detail) |
| `scene` | `string` | NO | -- | Add to proto |
| `provider` | `string` | NO | -- | Add to proto |
| `model` | `string` | YES | `model` | Match |
| `latency_ms` | `number` | YES | `latency_ms` | Match |
| `model_error` | `string` | NO | -- | Add to proto |
| `is_success` | `boolean` | NO | `status` (string) exists | Transform in handler |
| `agent_type` (proto) | -- | YES | `agent_type` | Not in frontend type |
| `tokens_used` (proto) | -- | YES | `tokens_used` | Not in frontend type |
| `status` (proto) | -- | YES | `status` | Frontend uses `is_success` boolean |

### AICallLogDetail (proto) vs AICallLogDetail (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| (extends AICallLogItem) | -- | -- | -- | -- |
| `updated_at` | `string` | NO | -- | Add to proto |
| `industry_id` | `number` | NO | -- | Add to proto |
| `prompt_source` | `string` | NO | -- | Add to proto |
| `selected_prompt_id` | `number` | NO | -- | Add to proto |
| `selected_prompt_name` | `string` | NO | -- | Add to proto |
| `rendered_prompt` | `string` | YES | `rendered_prompt` | Match |
| `request_messages` | `string` | YES | `request_messages` | Match |
| `runtime_config` | `string` | YES | `runtime_config` | Match |
| `scene_config` | `string` | NO | -- | Add to proto |
| `user_input` | `string` | YES | `user_input` | Match |
| `model_output` | `string` | YES | `model_output` | Match |
| `trace_id` | `string` | YES | `trace_id` | Match |
| `source` | `string` | YES | `source` | Match |
| `scene` | `string` | YES | `scene` | Match |
| `provider` | `string` | YES | `provider` | Match |
| `model` | `string` | YES | `model` | Match |
| `latency_ms` | `number` | YES | `latency_ms` | Match |
| `is_success` | `boolean` | YES | `is_success` | Match |
| `input_tokens` (proto) | -- | YES | `input_tokens` | Not in frontend type |
| `output_tokens` (proto) | -- | YES | `output_tokens` | Not in frontend type |
| `model_error` (proto) | -- | YES | `model_error` | Not in frontend type |

### Live2DModelInfo (proto) vs Live2DModel (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `name` | `string` | YES | `name` | Match |
| `industry_id` | `number \| null` | YES | `industry_id` | Match |
| `scene` | `string` | YES | `scene` | Match |
| `model_url` | `string` | YES | `model_url` | Match |
| `thumbnail_url` | `string` | YES | `thumbnail_url` | Match |
| `config_json` | `string` | YES | `config_json` | Match |
| `tts_config_id` | `number \| null` | YES | `tts_config_id` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `created_at` | `string` | YES | `created_at` | Match |
| `updated_at` | `string` | NO | -- | Add to proto |

### TTSConfigInfo (proto) vs TTSConfig (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `name` | `string` | YES | `name` | Match |
| `engine` | `string` | YES | `engine` | Match |
| `voice_id` | `string` | YES | `voice_id` | Match |
| `scene` | `string` | NO | -- | Add to proto |
| `auth_config_json` | `string` | YES | `auth_config_json` | Match |
| `params_json` | `string` | YES | `params_json` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `sort_order` | `number` | YES | `sort_order` | Match |
| `support_status` | `string` | NO | -- | Add to proto |
| `support_message` | `string` | NO | -- | Add to proto |
| `created_at` | `string` | YES | `created_at` | Match |
| `updated_at` | `string` | NO | -- | Add to proto |

### PromptTemplate (proto) vs PromptTemplate (frontend - PromptPage)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `industry_id` | `number \| null` | NO | `industry_code` exists | Transform in handler |
| `name` | `string` | YES | `name` | Match |
| `scene` | `string` | YES | `scene` | Match |
| `template_content` | `string` | NO | `content` exists | Transform in handler (rename) |
| `variables` | `string` | YES | `variables` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `created_at` | `string` | NO | -- | Add to proto |
| `updated_at` | `string` | YES | `updated_at` | Match |
| `industry_code` (proto) | -- | YES | `industry_code` | Frontend uses `industry_id` |
| `template_type` (proto) | -- | YES | `template_type` | Not in frontend type |

### IndustryInfo (admin.proto) vs Industry (frontend - TaxonomyPage)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `code` | `string` | YES | `code` | Match |
| `name` | `string` | YES | `name` | Match |
| `description` | `string` | YES | `description` | Match |
| `icon` | `string` | YES | `icon` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `sort_order` | `number` | YES | `sort_order` | Match |
| `created_at` (proto) | -- | YES | `created_at` | Not in frontend type |

### CategoryInfo (admin.proto) vs Category (frontend - TaxonomyPage)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `industry_id` | `number` | YES | `industry_id` | Match |
| `name` | `string` | YES | `name` | Match |
| `parent_id` | `number \| null` | YES | `parent_id` | Match |
| `sort_order` | `number` | YES | `sort_order` | Match |
| `icon` | `string` | YES | `icon` | Match |
| `description` | `string` | YES | `description` | Match |
| `created_at` (proto) | -- | YES | `created_at` | Not in frontend type |
| `children` (proto) | -- | YES | `children` | Not in frontend type |

### ScraperTaskDetail (proto) vs ScraperTaskDetail (frontend runtimeTypes)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `created_at` | `string` | YES | `created_at` | Match |
| `updated_at` | `string` | YES | `updated_at` | Match |
| `task_type` | `string` | YES | `task_type` | Match |
| `source_url` | `string` | YES | `source_url` | Match |
| `source_title` | `string` | YES | `source_title` | Match |
| `source` | `string` | YES | `source` | Match |
| `status` | `string` | YES | `status` | Match |
| `question_count` | `number` | YES | `question_count` | Match |
| `imported_count` | `number` | YES | `imported_count` | Match |
| `retry_count` | `number` | NO | -- | Add to proto |
| `error_msg` | `string` | YES | `error_msg` | Match |
| `payload_json` | `string` | NO | -- | Add to proto |
| `result_json` | `string` | YES | `result_json` | Match |
| `started_at` (proto) | -- | YES | `started_at` | Not in frontend type |
| `finished_at` (proto) | -- | YES | `finished_at` | Not in frontend type |

### ScraperSource (proto) vs ScraperSource (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `name` | `string` | YES | `name` | Match |
| `label` | `string` | YES | `label` | Match |
| `base_url` | `string` | YES | `base_url` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |

### PipelineCard (proto) vs QuestionPipelineCard (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `string` | YES | `id` | Match |
| `title` | `string` | YES | `title` | Match |
| `content` | `string` | YES | `content` | Match |
| `type` | `string` | YES | `type` | Match |
| `difficulty` | `string` | YES | `difficulty` | Match |
| `category` | `string` | YES | `category` | Match |
| `answer` | `string` | YES | `answer` | Match |
| `solution` | `string` | YES | `solution` | Match |
| `explanation` | `string` | YES | `explanation` | Match |
| `tags` | `string[]` | YES | `tags` | Match |
| `judge_config` | `QuestionJudgeConfig \| null` | NO | `judge_config` (string) | Transform in handler (string to object) |
| `confidence` | `number` | YES | `confidence` | Match |
| `source_type` | `string` | YES | `source_type` | Match |
| `source_label` | `string` | YES | `source_label` | Match |
| `source_title` | `string` | YES | `source_title` | Match |
| `source_url` | `string` | YES | `source_url` | Match |

### QuestionPipelineStats (frontend) vs `google.protobuf.Struct stats` (proto)

The proto returns stats as a generic `Struct`. The frontend expects typed fields:

| Frontend Field | Frontend Type | Proto Has It? | Action Needed |
|---|---|---|---|
| `searched_count` | `number` | NO (generic Struct) | Transform in handler |
| `fetched_count` | `number` | NO (generic Struct) | Transform in handler |
| `scraped_count` | `number` | NO (generic Struct) | Transform in handler |
| `generated_count` | `number` | NO (generic Struct) | Transform in handler |
| `candidate_count` | `number` | NO (generic Struct) | Transform in handler |
| `selected_sources` | `number` | NO (generic Struct) | Transform in handler |

### TagTaxonomyGroup (proto) vs QuestionTagTaxonomyGroup (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `group` | `string` | YES | `category` | Transform in handler (rename) |
| `description` | `string` | NO | -- | Add to proto |
| `tags` | `string[]` | YES | `tags` | Match |

### RAGDocumentDetail (proto) vs RAGDocument (frontend)

| Frontend Field | Frontend Type | Proto Has It? | Proto Field Name | Action Needed |
|---|---|---|---|---|
| `id` | `number` | YES | `id` | Match |
| `collection` | `string` | YES | `collection` | Match |
| `doc_type` | `string` | YES | `doc_type` | Match |
| `title` | `string` | YES | `title` | Match |
| `content` | `string` | YES | `content` | Match |
| `metadata` | `string` | YES | `metadata` | Match |
| `vector_id` | `string` | YES | `vector_id` | Match |
| `sync_status` | `string` | YES | `sync_status` | Match |
| `is_active` | `boolean` | YES | `is_active` | Match |
| `created_at` | `string` | YES | `created_at` | Match |
| `updated_at` | `string` | YES | `updated_at` | Match |

---

## 8. MEMBERSHIP SERVICE

**Proto file**: `D:\gogogo\makejob\api\makejob\membership\v1\membership.proto`
**Frontend**: Only found as a route reference in `D:\gogogo\makejob\frontend-react\apps\web\src\router.tsx`. No dedicated TypeScript types/interfaces found in the frontend codebase.

The membership service proto defines these messages: `MembershipStatus`, `MembershipPlan`, `OrderResponse`, `CheckFeatureResponse`. The frontend has **no corresponding TypeScript interfaces** -- the membership page either has not been built yet or types are inlined without separate definitions. There are **no mismatches to report** because the frontend consumer does not exist.

---

## SUMMARY OF CRITICAL MISMATCHES

The most impactful mismatches requiring immediate attention:

1. **Growth service**: The proto `GrowthSummary` has only 7 fields. The frontend `GrowthSummaryResponse` has 12 top-level fields with 7 completely new sub-message types (60+ new fields total). The proto is essentially a stub. The handler must be doing extensive aggregation from other services and the proto needs to be rewritten to match.

2. **Interview `CodingDiagnosis.evidence`**: Proto has `evidence_summary` as `string`, frontend expects `evidence` as `string[]` -- type mismatch that will cause runtime errors.

3. **Interview `InterviewDetail`**: Frontend expects `score`, `total_questions`, `current_question`, `started_at`, `ended_at`, `async_task_id`, `task_status`, `task_error` -- all missing from proto.

4. **Companion `CompanionPlanDetail`**: 12 fields the frontend expects are missing from `plan.proto`'s `PlanDetail` (phase tracking, industry info, async task, date ranges, adjustment history).

5. **Question `QuestionDetail`**: Frontend expects `solution`, `judge_config`, `answer_template` as structured objects. Proto only has `reference_answer`, `starter_code`, `test_cases` as flat fields. The structured sub-messages need to be added.

6. **Practice recommendations**: The proto `RecommendedQuestion` has only 5 fields. The frontend `PracticeRecommendationItem` has 17 fields including topic tracking, archive phases, question sets, and priority explanations.

7. **Community `PostSummary`**: Missing `post_type`, `summary`, `tags`, `view_count`, `is_pinned`, `is_recommended`, `updated_at`, `is_author`, and the `author` object composition.

8. **Admin `AdminConfigItem`**: Proto uses `key`/`value`, frontend expects `config_key`/`config_value` -- needs handler rename.

9. **Admin `AICallLog`**: Proto list-level message lacks `trace_id`, `source`, `scene`, `provider`, `model_error`, `is_success` -- these only exist in the detail message but the list frontend expects them.

10. **Question pipeline `judge_config`**: Proto stores it as a string (`judge_config` field in `PipelineCard`), frontend expects it as a parsed `QuestionJudgeConfig` object.