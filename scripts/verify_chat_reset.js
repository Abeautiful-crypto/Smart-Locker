const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 430, height: 860 } });
  const BASE = "http://localhost:8090/";
  // 登录
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900004444"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(1000);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  // 转人工
  await p.goto(BASE + "#u-chat"); await p.waitForTimeout(400);
  await p.click("#toHuman"); await p.waitForTimeout(500);
  const s1 = await p.evaluate(async () => {
    const tk = localStorage.getItem("locker_token");
    return (await (await fetch("/api/chat/state", { headers: { "X-Token": tk } })).json()).state;
  });
  console.log((s1 === "waiting" || s1 === "agent" ? "[PASS]" : "[FAIL]") + " 转人工后状态=" + s1);
  // 退出登录 → 重新登录
  await p.evaluate(() => localStorage.removeItem("locker_token"));
  await p.goto(BASE + "#u-login"); await p.reload(); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900004444"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(1000);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  const s2 = await p.evaluate(async () => {
    const tk = localStorage.getItem("locker_token");
    return (await (await fetch("/api/chat/state", { headers: { "X-Token": tk } })).json()).state;
  });
  console.log((s2 === "bot" ? "[PASS]" : "[FAIL]") + " 重新登录后状态=" + s2);
  await b.close();
  process.exit(s2 === "bot" ? 0 : 1);
})();
