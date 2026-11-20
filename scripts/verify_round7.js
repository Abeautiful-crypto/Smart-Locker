/* 第7轮：用户端去除取件入口 + 柜机结算提示补交 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (n, ok, d) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + n + "  " + d); };

(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 430, height: 860 } });
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message));

  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900001113"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(1800);

  // 用户端底部导航无"取件"
  await p.goto(BASE + "#u-home"); await p.waitForTimeout(400);
  const navText = await p.locator(".tabbar").innerText();
  check("Q2-底部导航无取件", !navText.includes("取件"), "nav=" + navText.replace(/\n/g, "/"));
  check("Q2-首页无我要取件", (await p.locator("body").innerText()).indexOf("我要取件") === -1, "no pickup button");

  // 直接访问 u-pickup 不会渲染取件页（回落到首页）
  await p.goto(BASE + "#u-pickup"); await p.waitForTimeout(400);
  check("Q2-直接访问u-pickup回落", (await p.evaluate(() => location.hash)) !== "#u-pickup" || !(await p.locator("#pickCode").count()), "hash=" + await p.evaluate(() => location.hash));

  // 订单卡片无"取件"按钮，有"补交"按钮（针对需补交订单）
  await p.goto(BASE + "#u-orders"); await p.waitForTimeout(500);
  check("Q2-订单无取件按钮", await p.locator(".op-pickup").count() === 0, "op-pickup=" + await p.locator(".op-pickup").count());

  // 柜机结算提示：用已存在的超时订单（code=815157，extra=3500）走公开结算
  const settleResult = await p.evaluate(async () => {
    const r = await fetch("/api/kiosk/pickup", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code: "815157" }) });
    if (!r.ok) return { pickup: false, err: (await r.json()).error };
    const pk = await r.json();
    const s = await fetch("/api/kiosk/orders/" + pk.orderId + "/pickup-done", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
    return { pickup: true, settle: await s.json() };
  });
  check("Q1-柜机结算返回extra", settleResult.pickup && Number(settleResult.settle.extra) > 0, "extra=" + (settleResult.settle && settleResult.settle.extra));

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
