$base = "http://localhost:8080"
$results = @()

function Test-API {
    param($Name, $Method, $Url, $Body, $Token)
    $headers = @{"Content-Type"="application/json"}
    if ($Token) { $headers["Authorization"] = "Bearer $Token" }
    try {
        if ($Method -eq "GET") {
            $resp = Invoke-RestMethod -Uri $Url -Method $Method -Headers $headers -ErrorAction Stop
        } else {
            $resp = Invoke-RestMethod -Uri $Url -Method $Method -Headers $headers -Body $Body -ErrorAction Stop
        }
        $code = if ($resp.code -ne $null) { $resp.code } else { 0 }
        $msg = if ($resp.message) { $resp.message } else { "ok" }
        $status = if ($code -eq 0) { "PASS" } else { "FAIL" }
        Write-Host "[$status] $Name -> code=$code msg=$msg"
        return @{Name=$Name; Status=$status; Code=$code; Message=$msg; Data=$resp.data}
    } catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        $errBody = ""
        try { 
            $reader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
            $errBody = $reader.ReadToEnd()
        } catch {}
        Write-Host "[FAIL] $Name -> HTTP $statusCode $errBody"
        return @{Name=$Name; Status="FAIL"; Code=$statusCode; Message=$errBody; Data=$null}
    }
}

Write-Host "========== MakeJob API Test Suite =========="
Write-Host ""

# 1. Health
$r = Test-API "Health Check" "GET" "$base/api/health"
$results += $r

# 2. Register (may fail if user exists)
$r = Test-API "Register" "POST" "$base/api/auth/register" '{"username":"testuser2","email":"test2@test.com","password":"test123456"}'
$results += $r

# 3. Login user
$r = Test-API "Login User" "POST" "$base/api/auth/login" '{"email":"test@test.com","password":"test123456"}'
$results += $r
$TOKEN = $r.Data.token
$REFRESH = $r.Data.refresh_token

# 4. Login admin
$r = Test-API "Login Admin" "POST" "$base/api/auth/login" '{"email":"admin@makejob.com","password":"admin123456"}'
$results += $r
$ADMIN_TOKEN = $r.Data.token

# 5. Refresh Token
$r = Test-API "Refresh Token" "POST" "$base/api/auth/refresh" "{`"refresh_token`":`"$REFRESH`"}"
$results += $r

# 6. Get Profile
$r = Test-API "Get Profile" "GET" "$base/api/user/profile" $null $TOKEN
$results += $r

Write-Host ""
Write-Host "========== Question APIs =========="

# 7. Categories
$r = Test-API "Categories" "GET" "$base/api/categories?industry_code=go" $null
$results += $r

# 8. Questions List
$r = Test-API "Questions List" "GET" "$base/api/questions?page=1&page_size=10" $null
$results += $r

# 9. Question Detail
$r = Test-API "Question Detail" "GET" "$base/api/questions/1" $null
$results += $r

# 10. Submit Answer
$r = Test-API "Submit Answer" "POST" "$base/api/questions/1/submit" '{"answer":"A","time_spent":30}' $TOKEN
$results += $r

# 11. Toggle Favorite
$r = Test-API "Toggle Favorite" "POST" "$base/api/questions/1/favorite" '{}' $TOKEN
$results += $r

# 12. Favorites List
$r = Test-API "Favorites List" "GET" "$base/api/user/favorites?page=1&page_size=10" $null $TOKEN
$results += $r

# 13. Wrong Questions
$r = Test-API "Wrong Questions" "GET" "$base/api/user/wrong-questions?page=1&page_size=10" $null $TOKEN
$results += $r

# 14. Create Note
$r = Test-API "Create Note" "POST" "$base/api/user/notes" '{"question_id":1,"title":"test note","content":"test content"}' $TOKEN
$results += $r

# 15. List Notes
$r = Test-API "List Notes" "GET" "$base/api/user/notes?page=1&page_size=10" $null $TOKEN
$results += $r

# 16. Random Exam
$r = Test-API "Random Exam" "POST" "$base/api/exams/random" '{"count":5,"difficulty":"easy"}' $TOKEN
$results += $r

# 17. Practice Stats
$r = Test-API "Practice Stats" "GET" "$base/api/user/practice-stats" $null $TOKEN
$results += $r

Write-Host ""
Write-Host "========== Interview APIs =========="

# 18. Create Interview
$r = Test-API "Create Interview" "POST" "$base/api/interviews" '{"industry_code":"go","difficulty":"medium","question_count":5}' $TOKEN
$results += $r
$interviewId = $r.Data.id
if (-not $interviewId) { $interviewId = 1 }

# 19. Interview List
$r = Test-API "Interview List" "GET" "$base/api/interviews?page=1&page_size=10" $null $TOKEN
$results += $r

# 20. Interview Detail
$r = Test-API "Interview Detail" "GET" "$base/api/interviews/$interviewId" $null $TOKEN
$results += $r

# 21. Submit Interview Answer
$r = Test-API "Interview Answer" "POST" "$base/api/interviews/$interviewId/answer" '{"answer":"goroutine is a lightweight thread in Go"}' $TOKEN
$results += $r

# 22. Get Next Question
$r = Test-API "Next Question" "GET" "$base/api/interviews/$interviewId/next" $null $TOKEN
$results += $r

# 23. Finish Interview
$r = Test-API "Finish Interview" "POST" "$base/api/interviews/$interviewId/finish" '{}' $TOKEN
$results += $r

# 24. Interview Report
$r = Test-API "Interview Report" "GET" "$base/api/interviews/$interviewId/report" $null $TOKEN
$results += $r

Write-Host ""
Write-Host "========== Plan APIs =========="

# 25. Generate Plan
$r = Test-API "Generate Plan" "POST" "$base/api/plans" '{"level":"beginner","daily_study_time":60,"duration_days":21,"goal_description":"Prepare for Go interview"}' $TOKEN
$results += $r
$planId = $r.Data.id
if (-not $planId) { $planId = 1 }

# 26. Current Plan
$r = Test-API "Current Plan" "GET" "$base/api/plans/current" $null $TOKEN
$results += $r

# 27. Plan Detail
$r = Test-API "Plan Detail" "GET" "$base/api/plans/$planId" $null $TOKEN
$results += $r

# 28. Update Task Status
$r = Test-API "Update Task" "PUT" "$base/api/plans/$planId/tasks/1" '{"status":"completed"}' $TOKEN
$results += $r

# 29. Plan Progress
$r = Test-API "Plan Progress" "GET" "$base/api/plans/$planId/progress" $null $TOKEN
$results += $r

Write-Host ""
Write-Host "========== Membership APIs =========="

# 30. Membership Plans
$r = Test-API "Membership Plans" "GET" "$base/api/membership/plans" $null $TOKEN
$results += $r

# 31. Create Order
$r = Test-API "Create Order" "POST" "$base/api/membership/orders" '{"plan_type":"monthly"}' $TOKEN
$results += $r
$orderNo = $r.Data.order_no

# 32. Order List
$r = Test-API "Order List" "GET" "$base/api/membership/orders?page=1&page_size=10" $null $TOKEN
$results += $r

# 33. Mock Payment (if order_no available)
if ($orderNo) {
    $r = Test-API "Mock Payment" "POST" "$base/api/membership/callback" "{`"order_no`":`"$orderNo`"}"
    $results += $r
} else {
    Write-Host "[SKIP] Mock Payment - no order_no"
}

# 34. Membership Status
$r = Test-API "Membership Status" "GET" "$base/api/membership/status" $null $TOKEN
$results += $r

Write-Host ""
Write-Host "========== Admin APIs =========="

# 35. Dashboard
$r = Test-API "Admin Dashboard" "GET" "$base/api/admin/dashboard" $null $ADMIN_TOKEN
$results += $r

# 36. Admin Users
$r = Test-API "Admin Users" "GET" "$base/api/admin/users?page=1&page_size=10" $null $ADMIN_TOKEN
$results += $r

# 37. Admin Questions
$r = Test-API "Admin Questions" "GET" "$base/api/admin/questions?page=1&page_size=10" $null $ADMIN_TOKEN
$results += $r

# 38. Admin Industries
$r = Test-API "Admin Industries" "GET" "$base/api/admin/industries" $null $ADMIN_TOKEN
$results += $r

# 39. Admin Prompts
$r = Test-API "Admin Prompts" "GET" "$base/api/admin/prompts" $null $ADMIN_TOKEN
$results += $r

# 40. Admin AI Configs
$r = Test-API "Admin AI Configs" "GET" "$base/api/admin/ai-configs" $null $ADMIN_TOKEN
$results += $r

# 41. Admin Live2D Models
$r = Test-API "Admin Live2D" "GET" "$base/api/admin/live2d-models" $null $ADMIN_TOKEN
$results += $r

# 42. Admin TTS Configs
$r = Test-API "Admin TTS" "GET" "$base/api/admin/tts-configs" $null $ADMIN_TOKEN
$results += $r

# 43. Scraper Sources
$r = Test-API "Scraper Sources" "GET" "$base/api/admin/scraper/sources" $null $ADMIN_TOKEN
$results += $r

Write-Host ""
Write-Host "=========================================="
Write-Host "SUMMARY"
Write-Host "=========================================="
$pass = ($results | Where-Object { $_.Status -eq "PASS" }).Count
$fail = ($results | Where-Object { $_.Status -eq "FAIL" }).Count
$total = $results.Count
Write-Host "Total: $total | PASS: $pass | FAIL: $fail"
Write-Host ""
if ($fail -gt 0) {
    Write-Host "Failed APIs:"
    $results | Where-Object { $_.Status -eq "FAIL" } | ForEach-Object {
        Write-Host "  - $($_.Name): code=$($_.Code) msg=$($_.Message)"
    }
}
