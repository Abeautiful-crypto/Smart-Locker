# 端到端 API 冒烟
$base = "http://localhost:8090"
function Post($path, $body, $token) {
  $h = @{ "Content-Type" = "application/json" }
  if ($token) { $h["X-Token"] = $token }
  try { Invoke-RestMethod -Uri "$base$path" -Method Post -Headers $h -Body ($body | ConvertTo-Json -Depth 5) } catch { "ERR: " + $_.Exception.Message + " | " + $_.ErrorDetails.Message }
}
function Get2($path, $token) {
  $h = @{}
  if ($token) { $h["X-Token"] = $token }
  try { Invoke-RestMethod -Uri "$base$path" -Method Get -Headers $h } catch { "ERR: " + $_.Exception.Message + " | " + $_.ErrorDetails.Message }
}
"== 用户登录 =="
$login = Post "/api/user/login" @{ phone = "13900001111"; code = "1234"; agree = $true }
$login | ConvertTo-Json -Compress
$tk = $login.token
"== 网点列表 =="
$outs = Get2 "/api/outlets?city=上海"
$outs | Select-Object -First 2 | ConvertTo-Json -Compress
$outletId = $outs[0].id
"== 试算 =="
Post "/api/quote" @{ outletId = $outletId; size = "S"; payMode = "prepaid" } | ConvertTo-Json -Compress
"== 下单 =="
$order = Post "/api/orders" @{ outletId = $outletId; size = "S"; payMode = "prepaid" } $tk
$order | ConvertTo-Json -Compress
"== 取件码取件 =="
Post "/api/orders/$($order.orderId)/pickup" @{ method = "code"; code = $order.code } $tk | ConvertTo-Json -Compress
"== 关门结算（免费时长内）=="
Post "/api/orders/$($order.orderId)/pickup-done" @{} $tk | ConvertTo-Json -Compress
"== 后台登录(超管) =="
$admin = Post "/api/admin/login" @{ identity = "super"; password = "000000" }
$admin | ConvertTo-Json -Compress
"== 看板 =="
Get2 "/api/admin/dashboard?range=today" $admin.token | ConvertTo-Json -Compress
"== 客服登录越权访问员工管理（应403）=="
$svc = Post "/api/admin/login" @{ identity = "svc"; password = "000000" }
Get2 "/api/admin/staff" $svc.token | ConvertTo-Json -Compress
"== 客服下班开关+工单 =="
Post "/api/admin/cs/duty" @{ on = $false } $svc.token | ConvertTo-Json -Compress
Post "/api/chat/send" @{ content = "我的快递丢失了怎么办啊" } $tk | ConvertTo-Json -Compress
"== 知识库命中 =="
Post "/api/chat/send" @{ content = "寄存怎么收费" } $tk | ConvertTo-Json -Compress
