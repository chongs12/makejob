@echo off
setlocal enabledelayedexpansion

echo ============================================
echo MakeJob API Test Suite
echo ============================================

set BASE=http://localhost:8080

echo.
echo [1] Health Check
curl -s %BASE%/api/health
echo.

echo.
echo [2] Login as testuser
for /f "delims=" %%i in ('curl -s -X POST %BASE%/api/auth/login -H "Content-Type: application/json" -d "{\"email\":\"test@test.com\",\"password\":\"test123456\"}"') do set LOGIN_RESP=%%i
echo %LOGIN_RESP%

REM Extract token using PowerShell
for /f "delims=" %%t in ('powershell -Command "('%LOGIN_RESP%' | ConvertFrom-Json).data.token"') do set TOKEN=%%t
for /f "delims=" %%t in ('powershell -Command "('%LOGIN_RESP%' | ConvertFrom-Json).data.refresh_token"') do set REFRESH_TOKEN=%%t
echo TOKEN=%TOKEN:~0,30%...

echo.
echo [3] Login as admin
for /f "delims=" %%i in ('curl -s -X POST %BASE%/api/auth/login -H "Content-Type: application/json" -d "{\"email\":\"admin@makejob.com\",\"password\":\"admin123456\"}"') do set ADMIN_RESP=%%i
echo %ADMIN_RESP%
for /f "delims=" %%t in ('powershell -Command "('%ADMIN_RESP%' | ConvertFrom-Json).data.token"') do set ADMIN_TOKEN=%%t
echo ADMIN_TOKEN=%ADMIN_TOKEN:~0,30%...

echo.
echo [4] Token Refresh
curl -s -X POST %BASE%/api/auth/refresh -H "Content-Type: application/json" -d "{\"refresh_token\":\"%REFRESH_TOKEN%\"}"
echo.

echo.
echo [5] Get Profile
curl -s -X GET %BASE%/api/user/profile -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo ============ Question APIs ============

echo.
echo [6] Categories
curl -s "%BASE%/api/categories?industry_code=go"
echo.

echo.
echo [7] Questions List
curl -s "%BASE%/api/questions?page=1&page_size=10"
echo.

echo.
echo [8] Question Detail
curl -s "%BASE%/api/questions/1"
echo.

echo.
echo [9] Submit Answer
curl -s -X POST %BASE%/api/questions/1/submit -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"answer\":\"A\",\"time_spent\":30}"
echo.

echo.
echo [10] Toggle Favorite
curl -s -X POST %BASE%/api/questions/1/favorite -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [11] Favorites List
curl -s "%BASE%/api/user/favorites?page=1&page_size=10" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [12] Wrong Questions
curl -s "%BASE%/api/user/wrong-questions?page=1&page_size=10" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [13] Create Note
curl -s -X POST %BASE%/api/user/notes -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"question_id\":1,\"title\":\"test note\",\"content\":\"this is a test note\"}"
echo.

echo.
echo [14] List Notes
curl -s "%BASE%/api/user/notes?page=1&page_size=10" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [15] Random Exam
curl -s -X POST %BASE%/api/exams/random -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"count\":5,\"difficulty\":\"easy\"}"
echo.

echo.
echo [16] Practice Stats
curl -s "%BASE%/api/user/practice-stats" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo ============ Interview APIs ============

echo.
echo [17] Create Interview
for /f "delims=" %%i in ('curl -s -X POST %BASE%/api/interviews -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"industry_code\":\"go\",\"difficulty\":\"medium\",\"question_count\":5}"') do set INTERVIEW_RESP=%%i
echo %INTERVIEW_RESP%

echo.
echo [18] Interview List
curl -s "%BASE%/api/interviews?page=1&page_size=10" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [19] Interview Detail
curl -s "%BASE%/api/interviews/1" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [20] Submit Interview Answer
curl -s -X POST %BASE%/api/interviews/1/answer -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"answer\":\"goroutine is a lightweight thread in Go\"}"
echo.

echo.
echo [21] Get Next Question
curl -s "%BASE%/api/interviews/1/next" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [22] Finish Interview
curl -s -X POST %BASE%/api/interviews/1/finish -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [23] Interview Report
curl -s "%BASE%/api/interviews/1/report" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo ============ Plan APIs ============

echo.
echo [24] Generate Plan
for /f "delims=" %%i in ('curl -s -X POST %BASE%/api/plans -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"level\":\"beginner\",\"daily_study_time\":60,\"duration_days\":21,\"goal_description\":\"Prepare for Go interview\"}"') do set PLAN_RESP=%%i
echo %PLAN_RESP%

echo.
echo [25] Current Plan
curl -s "%BASE%/api/plans/current" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [26] Plan Detail
curl -s "%BASE%/api/plans/1" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [27] Update Task Status
curl -s -X PUT %BASE%/api/plans/1/tasks/1 -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"status\":\"completed\"}"
echo.

echo.
echo [28] Plan Progress
curl -s "%BASE%/api/plans/1/progress" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo ============ Membership APIs ============

echo.
echo [29] Membership Plans
curl -s "%BASE%/api/membership/plans" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [30] Create Order
for /f "delims=" %%i in ('curl -s -X POST %BASE%/api/membership/orders -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"plan_type\":\"monthly\"}"') do set ORDER_RESP=%%i
echo %ORDER_RESP%

echo.
echo [31] Order List
curl -s "%BASE%/api/membership/orders?page=1&page_size=10" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo [32] Membership Status
curl -s "%BASE%/api/membership/status" -H "Authorization: Bearer %TOKEN%"
echo.

echo.
echo ============ Admin APIs ============

echo.
echo [33] Admin Dashboard
curl -s "%BASE%/api/admin/dashboard" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [34] Admin Users
curl -s "%BASE%/api/admin/users?page=1&page_size=10" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [35] Admin Questions
curl -s "%BASE%/api/admin/questions?page=1&page_size=10" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [36] Admin Industries
curl -s "%BASE%/api/admin/industries" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [37] Admin Prompts
curl -s "%BASE%/api/admin/prompts" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [38] Admin AI Configs
curl -s "%BASE%/api/admin/ai-configs" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [39] Admin Live2D Models
curl -s "%BASE%/api/admin/live2d-models" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [40] Admin TTS Configs
curl -s "%BASE%/api/admin/tts-configs" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo [41] Admin Scraper Sources
curl -s "%BASE%/api/admin/scraper/sources" -H "Authorization: Bearer %ADMIN_TOKEN%"
echo.

echo.
echo ============================================
echo Test Suite Complete
echo ============================================
