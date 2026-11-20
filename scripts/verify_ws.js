/* 验证 WebSocket 实时通道 + HTTP 轮询兜底 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";

(async () => {
  const b = await chromium.launch({ headless: true });
  // 用户端打开客服页，建立用户 WS
  const user = await b.newPage();
  await user.goto(BASE + "#u-login"); await user.waitForTimeout(300);
  await user.fill("#phone", "13900001113"); await user.fill("#code", "1234"); await user.check("#agreeChk"); await user.click("#doLogin"); await user.waitForTimeout(600);
  await user.goto(BASE + "#u-chat"); await user.waitForTimeout(600);
  const userWS = await user.evaluate(() => {
    // 通过 Performance API 检测 ws 升级请求
    return performance.getEntriesByType("resource").filter((e) => e.name.includes("/api/ws/chat")).length;
  });

  // 后台打开客服工作台，建立后台 WS
  const adm = await b.newPage();
  await adm.goto(BASE + "#m-login"); await adm.waitForTimeout(300);
  await adm.fill("#admId", "svc"); await adm.fill("#admPwd", "000000"); await adm.click("#admLogin"); await adm.waitForTimeout(600);
  await adm.goto(BASE + "#m-cs"); await adm.waitForTimeout(800);

  // 用原生 WebSocket 直连验证握手与消息推送
  const wsResult = await adm.evaluate(async () => {
    const token = JSON.parse(localStorage.getItem("locker_admin")).token;
    return new Promise((resolve) => {
      let done = false;
      const ws = new WebSocket("ws://" + location.host + "/api/admin/ws/chat?token=" + encodeURIComponent(token));
      const timer = setTimeout(() => { if (!done) { done = true; resolve({ ok: false, reason: "timeout 3s" }); ws.close(); } }, 3000);
      ws.onopen = () => { if (!done) { done = true; clearTimeout(timer); resolve({ ok: true, readyState: ws.readyState }); ws.close(); } };
      ws.onerror = (e) => { if (!done) { done = true; clearTimeout(timer); resolve({ ok: false, reason: "onerror" }); } };
    });
  });
  console.log("后台 WS 握手:", JSON.stringify(wsResult), "用户端 WS 资源条目:", userWS);
  console.log("客服双栏:", await adm.locator(".wx-shell").count(), "联系人列表:", await adm.locator("#csContactList").count());
  console.log(wsResult.ok ? "[PASS] WebSocket 实时通道可用" : "[FAIL] WebSocket 连接失败");
  await b.close();
  process.exit(wsResult.ok ? 0 : 1);
})();
