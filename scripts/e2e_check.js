/* 前端 e2e 验证：登录→存件→支付→取件→结算；后台看板 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));

const BASE = "http://localhost:8090";
const SHOT = "c:/Users/91526/Desktop/智能寄存柜/Smart-Locker/shots";

(async () => {
  const fs = require("fs");
  fs.mkdirSync(SHOT, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 430, height: 860 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
  page.on("console", (m) => { if (m.type() === "error") errors.push("console: " + m.text()); });

  const shot = async (n) => page.screenshot({ path: SHOT + "/" + n + ".png" });

  // 1. 首页
  await page.goto(BASE + "/#u-home", { waitUntil: "networkidle" }).catch(() => {});
  await page.waitForTimeout(600);
  await shot("01-home");
  const hero = await page.textContent(".hero-card .big").catch(() => null);
  console.log("首页空闲格:", hero);

  // 2. 登录
  await page.goto(BASE + "/#u-login", { waitUntil: "load" });
  await page.fill("#phone", "13911112222");
  await page.fill("#code", "1234");
  await page.check("#agreeChk");
  await page.click("#doLogin");
  await page.waitForTimeout(700);
  console.log("登录后hash:", await page.evaluate(() => location.hash));

  // 3. 选择网点→规格
  await page.goto(BASE + "/#u-outlets", { waitUntil: "load" });
  await page.waitForTimeout(500);
  await shot("02-outlets");
  await page.click('[data-action="outlet"]');
  await page.waitForTimeout(500);
  await page.goto(BASE + "/#u-spec", { waitUntil: "load" });
  await page.waitForTimeout(500);
  await page.click('[data-size="S"]');
  await page.waitForTimeout(300);
  await page.click("#toPay");
  await page.waitForTimeout(600);
  await shot("03-pay");
  const payTxt = await page.textContent("#doPay").catch(() => "N/A");
  console.log("支付按钮:", payTxt);

  // 4. 支付下单
  await page.click("#doPay");
  await page.waitForTimeout(800);
  await shot("04-open");
  const code = await page.textContent("[style*='letter-spacing']").catch(() => null);
  console.log("取件码页:", code, "hash:", await page.evaluate(() => location.hash));

  // 5. 订单页取件→结算
  await page.goto(BASE + "/#u-orders", { waitUntil: "load" });
  await page.waitForTimeout(600);
  await shot("05-orders");
  await page.click(".op-done");
  await page.waitForTimeout(800);
  await shot("06-settled");
  console.log("结算后订单页hash:", await page.evaluate(() => location.hash));

  // 6. 后台登录与看板
  const admin = await browser.newPage({ viewport: { width: 1200, height: 800 } });
  admin.on("pageerror", (e) => errors.push("admin pageerror: " + e.message));
  await admin.goto(BASE + "/#m-login", { waitUntil: "load" });
  await admin.fill("#admId", "super");
  await admin.fill("#admPwd", "000000");
  await admin.click("#admLogin");
  await admin.waitForTimeout(800);
  await admin.screenshot({ path: SHOT + "/07-admin-dash.png" });
  console.log("后台hash:", await admin.evaluate(() => location.hash));
  // 网点管理
  await admin.goto(BASE + "/#m-outlets", { waitUntil: "load" });
  await admin.waitForTimeout(600);
  await admin.screenshot({ path: SHOT + "/08-admin-outlets.png" });
  // 柜格监控
  await admin.goto(BASE + "/#m-monitor", { waitUntil: "load" });
  await admin.waitForTimeout(600);
  await admin.screenshot({ path: SHOT + "/09-admin-monitor.png" });
  // 客服工作台（svc）
  await admin.evaluate(() => localStorage.removeItem("locker_admin"));
  await admin.goto(BASE + "/#m-login", { waitUntil: "load" });
  await admin.fill("#admId", "svc");
  await admin.fill("#admPwd", "000000");
  await admin.click("#admLogin");
  await admin.waitForTimeout(800);
  await admin.goto(BASE + "/#m-cs", { waitUntil: "load" });
  await admin.waitForTimeout(600);
  await admin.screenshot({ path: SHOT + "/10-admin-cs.png" });

  // 7. 柜机端
  await page.goto(BASE + "/#k-standby", { waitUntil: "load" });
  await page.waitForTimeout(500);
  await shot("11-kiosk");

  console.log("JS错误:", errors.length ? errors.join("\n") : "无");
  await browser.close();
})();
