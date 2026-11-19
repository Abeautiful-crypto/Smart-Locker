# Five-scenario HTTP verification (independent test evidence, run against live backend :8090)
# Usage: powershell -File scripts/five_scenarios.ps1  (requires local MySQL CLI for S5 setup)
$ErrorActionPreference = "Stop"
$base = "http://localhost:8090"
$mysql = "D:\phpstudy_pro\Extensions\MySQL8.0.12\bin\mysql.exe"
$pass = 0; $fail = 0
function Check($name, $cond, $detail) {
  if ($cond) { $script:pass++; Write-Output ("[PASS] " + $name + "  " + $detail) }
  else { $script:fail++; Write-Output ("[FAIL] " + $name + "  " + $detail) }
}
function PostJson($path, $json, $token) {
  $h = @{ "Content-Type" = "application/json; charset=utf-8" }
  if ($token) { $h["X-Token"] = $token }
  try {
    $r = Invoke-RestMethod -Uri "$base$path" -Method Post -Headers $h -Body ([System.Text.Encoding]::UTF8.GetBytes($json))
    return @{ code = 200; body = $r }
  } catch {
    $sc = $_.Exception.Response.StatusCode.value__
    return @{ code = $sc; body = $null }
  }
}
function GetJson($path, $token) {
  $h = @{}
  if ($token) { $h["X-Token"] = $token }
  try {
    $r = Invoke-RestMethod -Uri "$base$path" -Method Get -Headers $h
    return @{ code = 200; body = $r }
  } catch {
    $sc = $_.Exception.Response.StatusCode.value__
    return @{ code = $sc; body = $null }
  }
}
function LoginUser($phone) {
  $r = PostJson "/api/user/login" ('{"phone":"' + $phone + '","code":"1234","agree":true}') $null
  return $r.body.token
}
function LoginAdmin($identity) {
  $r = PostJson "/api/admin/login" ('{"identity":"' + $identity + '","password":"000000"}') $null
  return $r.body.token
}

# ---------- S1 normal order loop ----------
$tk1 = LoginUser "13900001113"
$before = (GetJson "/api/outlets/1" $null).body
$idleBefore = $before.byStatus.idle
$o1 = (PostJson "/api/orders" '{"outletId":1,"size":"S","payMode":"prepaid"}' $tk1).body
Check "S1-order-prepaid" ($o1.paid -eq 1500) ("paid=" + $o1.paid)
$pk = PostJson "/api/orders/$($o1.orderId)/pickup" ('{"method":"code","code":"' + $o1.code + '"}') $tk1
Check "S1-pickup-code" ($pk.code -eq 200) ("http=" + $pk.code)
$done1 = (PostJson "/api/orders/$($o1.orderId)/pickup-done" '{}' $tk1).body
Check "S1-settle-free" ($done1.kind -eq "free" -and $done1.fee -eq 0 -and $done1.refund -eq 1500) ("kind=$($done1.kind) fee=$($done1.fee) refund=$($done1.refund)")
$od1 = (GetJson "/api/orders/$($o1.orderId)" $tk1).body
Check "S1-order-finished" ($od1.status -eq "finished") ("status=" + $od1.status)
$after = (GetJson "/api/outlets/1" $null).body
Check "S1-locker-released" ($after.byStatus.idle -eq $idleBefore) ("idle before=$idleBefore after=$($after.byStatus.idle)")

# ---------- S2 free-window full refund (independent user) ----------
$tk2 = LoginUser "13900001114"
$o2 = (PostJson "/api/orders" '{"outletId":2,"size":"M","payMode":"prepaid"}' $tk2).body
Check "S2-prepaid-M" ($o2.paid -eq 2700) ("paid=" + $o2.paid)
PostJson "/api/orders/$($o2.orderId)/pickup" ('{"method":"code","code":"' + $o2.code + '"}') $tk2 | Out-Null
$done2 = (PostJson "/api/orders/$($o2.orderId)/pickup-done" '{}' $tk2).body
Check "S2-free-fullrefund" ($done2.fee -eq 0 -and $done2.refund -eq 2700) ("fee=$($done2.fee) refund=$($done2.refund)")

# ---------- S3 report problem -> full refund + locker fault + alert ----------
$tk3 = LoginUser "13900001115"
$o3 = (PostJson "/api/orders" '{"outletId":1,"size":"S","payMode":"prepaid"}' $tk3).body
$rep = PostJson "/api/orders/$($o3.orderId)/report" '{"problem":"cabindoor"}' $tk3
Check "S3-report-ok" ($rep.code -eq 200) ("http=" + $rep.code)
$od3 = (GetJson "/api/orders/$($o3.orderId)" $tk3).body
Check "S3-full-refund-finished" ($od3.status -eq "finished" -and $od3.fee -eq 0 -and $od3.refund -eq 1500) ("status=$($od3.status) fee=$($od3.fee) refund=$($od3.refund)")
$suTk = LoginAdmin "super"
$alerts = (GetJson "/api/admin/alerts?status=open" $suTk).body.alerts
$hasAlert = $false
foreach ($a in @($alerts)) { if ($a.source -eq "user_report") { $hasAlert = $true } }
Check "S3-alert-created" $hasAlert ("alerts=" + @($alerts).Count)
$lkFault = (GetJson "/api/admin/lockers?outletId=1" $suTk).body | Where-Object { $_.id -eq $o3.lockerId }
Check "S3-locker-fault" ($lkFault.status -eq "fault") ("locker status=" + $lkFault.status)

# ---------- S4 authorization ----------
$anon = GetJson "/api/orders/current" $null
Check "S4-anon-401" ($anon.code -eq 401) ("http=" + $anon.code)
$uAdmin = GetJson "/api/admin/dashboard" $tk1
Check "S4-user-to-admin-denied" ($uAdmin.code -eq 401 -or $uAdmin.code -eq 403) ("http=" + $uAdmin.code)
$svcTk = LoginAdmin "svc"
$svcStaff = GetJson "/api/admin/staff" $svcTk
Check "S4-service-no-staff-403" ($svcStaff.code -eq 403) ("http=" + $svcStaff.code)
$siteTk = LoginAdmin "site_rm"
# 权限矩阵：收费规则配置仅超管，网点管理员不可访问（含本网点）
$rOwn = GetJson "/api/admin/rules/1" $siteTk
Check "S4-site-no-rule-403" ($rOwn.code -eq 403) ("http=" + $rOwn.code)
# 网点管理员可访问本网点订单
$ordOwn = GetJson "/api/admin/orders?outletId=1" $siteTk
Check "S4-site-own-orders-200" ($ordOwn.code -eq 200) ("http=" + $ordOwn.code)

# ---------- S5 coupon settle ----------
$tk5 = LoginUser "13900001116"
$uid = (GetJson "/api/user/me" $tk5).body.userId
$ErrorActionPreference = "Continue"
cmd /c "`"$mysql`" -uroot -proot smart_locker --default-character-set=utf8mb4 -N -e `"INSERT INTO coupon_instances(def_id,user_id,status) SELECT id,$uid,'usable' FROM coupon_defs WHERE ctype='discount' AND enabled=1 AND CURDATE() BETWEEN valid_from AND valid_to LIMIT 1;`" 2>nul"
$ErrorActionPreference = "Stop"
$cps = (GetJson "/api/user/coupons" $tk5).body.usable
Check "S5-coupon-granted" (@($cps).Count -ge 1) ("usable=" + @($cps).Count)
$cid = @($cps)[0].id
$o5 = (PostJson "/api/orders" ('{"outletId":1,"size":"S","payMode":"prepaid","couponInstanceId":' + $cid + '}') $tk5).body
$od5 = (GetJson "/api/orders/$($o5.orderId)" $tk5).body
Check "S5-order-with-coupon" ($od5.couponInstanceId -eq $cid) ("couponInstanceId=" + $od5.couponInstanceId)
PostJson "/api/orders/$($o5.orderId)/pickup" ('{"method":"code","code":"' + $o5.code + '"}') $tk5 | Out-Null
$done5 = (PostJson "/api/orders/$($o5.orderId)/pickup-done" '{}' $tk5).body
Check "S5-settle-cap" (($done5.fee - $done5.coupon) -ge 0 -and $done5.coupon -ge 0) ("fee=$($done5.fee) coupon=$($done5.coupon) refund=$($done5.refund)")
# 免费时长内费用为0：优惠额封顶为0，券不核销（不浪费券）；核销路径由单测覆盖（65分钟场景）
$cps2 = (GetJson "/api/user/coupons" $tk5).body
$stillUsable = $false
foreach ($c in @($cps2.usable)) { if ($c.id -eq $cid) { $stillUsable = $true } }
Check "S5-coupon-not-wasted" ($stillUsable) ("coupon kept usable when fee=0")

Write-Output ""
Write-Output ("RESULT: PASS=" + $pass + " FAIL=" + $fail)
if ($fail -gt 0) { exit 1 }
