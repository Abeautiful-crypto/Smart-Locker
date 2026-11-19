# 存取件闭环冒烟（UTF-8 字节体）
$base = "http://localhost:8090"
function PostJson($path, $json, $token) {
  $h = @{ "Content-Type" = "application/json; charset=utf-8" }
  if ($token) { $h["X-Token"] = $token }
  Invoke-RestMethod -Uri "$base$path" -Method Post -Headers $h -Body ([System.Text.Encoding]::UTF8.GetBytes($json))
}
$login = PostJson "/api/user/login" '{"phone":"13900001112","code":"1234","agree":true}'
"登录: " + ($login | ConvertTo-Json -Compress)
$order = PostJson "/api/orders" '{"outletId":1,"size":"S","payMode":"prepaid"}' $login.token
"下单: " + ($order | ConvertTo-Json -Compress)
$pk = PostJson "/api/orders/$($order.orderId)/pickup" ('{"method":"code","code":"' + $order.code + '"}') $login.token
"取件验证: " + ($pk | ConvertTo-Json -Compress)
$done = PostJson "/api/orders/$($order.orderId)/pickup-done" '{}' $login.token
"结算: " + ($done | ConvertTo-Json -Compress)
$cur = Invoke-RestMethod -Uri "$base/api/orders/current" -Headers @{"X-Token"=$login.token}
"当前订单数: " + $cur.Count
$hist = Invoke-RestMethod -Uri "$base/api/orders/history" -Headers @{"X-Token"=$login.token}
"历史订单数: " + $hist.Count
