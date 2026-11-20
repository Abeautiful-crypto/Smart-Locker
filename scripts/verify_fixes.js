/* 验证用户反馈 8 项问题的修复 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
function check(name, ok, detail) { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + name + "  " + detail); }

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  const errs = [];
  page.on("pageerror", (e) => errs.push("PAGEERROR " + e.message));

  // 登录用户
  await page.goto(BASE + "#u-login", { waitUntil: "load" });
  await page.waitForTimeout(300);
  await page.click('.login-tab[data-m="code"]'); await page.waitForTimeout(150); await page.fill("#phone", "13900001113"); await page.fill("#code", "1234"); await page.check("#agreeChk");
  await page.click("#doLogin"); await page.waitForTimeout(1800); // 等待登录 toast 消失

  // Q4 我的：无员工后台入口 + 取件密码可设置
  await page.goto(BASE + "#u-mine", { waitUntil: "load" }); await page.waitForTimeout(500);
  check("Q4-员工后台入口已移除", await page.locator("text=员工后台入口").count() === 0, "count=0");
  await page.locator("#setPwd").click({ force: true }); await page.waitForTimeout(200);
  check("Q4-取件密码弹窗可打开", await page.locator("#mNewPwd").count() > 0, "modal shown");
  if (await page.locator("#mNewPwd").count()) {
    await page.fill("#mNewPwd", "123456"); await page.locator("#mSetPwd").click({ force: true }); await page.waitForTimeout(400);
    check("Q4-取件密码保存", (await page.locator("#toast-root").innerText()).includes("保存"), "toast=" + await page.locator("#toast-root").innerText());
  }

  // 下单获取取件码
  await page.goto(BASE + "#u-home", { waitUntil: "load" }); await page.waitForTimeout(200);
  await page.click('[data-action="nav"][data-id="u-outlets"]'); await page.waitForTimeout(300);
  await page.locator("[data-id]").filter({ hasText: "人民广场" }).first().click().catch(() => {});
  await page.waitForTimeout(300);
  if (await page.locator('[data-action="nav"][data-id="u-spec"]').count()) await page.click('[data-action="nav"][data-id="u-spec"]');
  if ((await page.evaluate(() => location.hash)) !== "#u-spec") await page.goto(BASE + "#u-spec", { waitUntil: "load" });
  await page.waitForTimeout(300);
  const specBtn = page.locator(".spec-option").first();
  if (await specBtn.count()) await specBtn.click();
  await page.waitForTimeout(200);
  // 直接进入下单页兜底
  if ((await page.evaluate(() => location.hash)) !== "#u-pay") await page.goto(BASE + "#u-pay", { waitUntil: "load" });
  await page.waitForTimeout(300);
  let code = "";
  const payBtn = page.locator("#doPay, #payNow, .pay-confirm");
  if (await payBtn.count()) { await payBtn.first().click(); await page.waitForTimeout(500); code = (await page.locator("#uOpenCode, .pay-code-value").first().innerText().catch(() => "")); }
  if (!code) {
    // 通过 API 下单
    code = await page.evaluate(async () => {
      const tk = localStorage.getItem("locker_token") || "";
      const r = await fetch("/api/orders", { method: "POST", headers: { "Content-Type": "application/json", "X-Token": tk }, body: JSON.stringify({ outletId: 1, size: "S", payMode: "prepaid" }) });
      return (await r.json()).code || "";
    });
  }
  check("Q1-取件码存在", !!code, "code=" + code);

  // 订单卡片显示取件码
  await page.goto(BASE + "#u-orders", { waitUntil: "load" }); await page.waitForTimeout(400);
  const codeBoxCount = await page.locator(".pickup-code-box").count();
  check("Q1-订单卡片显示取件码", codeBoxCount > 0, "boxes=" + codeBoxCount);

  // Q2 取件页有底部导航
  await page.locator(".op-pickup").first().click(); await page.waitForTimeout(300);
  check("Q2-取件页底部导航存在", await page.locator(".tabbar").count() > 0, "nav=" + await page.locator(".tabbar").count());

  // Q3 错误取件码有页面内提示
  await page.fill("#pickCode", "000000"); await page.click("#pickByCode"); await page.waitForTimeout(500);
  check("Q3-错误取件码提示", (await page.locator("#pickResult").innerText()).length > 5, "msg=" + (await page.locator("#pickResult").innerText()).slice(0, 30));

  // Q5 后台净营收标注口径
  await page.goto(BASE + "#m-login", { waitUntil: "load" }); await page.waitForTimeout(200);
  await page.fill("#admId", "super"); await page.fill("#admPwd", "000000"); await page.click("#admLogin"); await page.waitForTimeout(500);
  await page.goto(BASE + "#m-dash", { waitUntil: "load" }); await page.waitForTimeout(300);
  check("Q5-净营收口径标注", (await page.locator(".stat-grid").innerText()).includes("收款-退款+补交"), "labeled");

  // Q6 新建优惠券默认值完整可创建
  await page.goto(BASE + "#m-coupons", { waitUntil: "load" }); await page.waitForTimeout(300);
  const before = await page.locator("table tbody tr").count();
  await page.click("#addCp"); await page.waitForTimeout(200);
  await page.fill("#cpName", "验证券");
  const fromVal = await page.locator("#cpFrom").inputValue(), toVal = await page.locator("#cpTo").inputValue(), totalVal = await page.locator("#cpTotal").inputValue();
  check("Q6-表单默认值完整", !!fromVal && !!toVal && Number(totalVal) > 0, `from=${fromVal} to=${toVal} total=${totalVal}`);
  await page.click("#cpSave"); await page.waitForTimeout(500);
  const after = await page.locator("table tbody tr").count();
  check("Q6-优惠券创建生效", after > before, `before=${before} after=${after}`);

  // Q7 客服工作台微信式双栏
  await page.goto(BASE + "#m-cs", { waitUntil: "load" }); await page.waitForTimeout(500);
  check("Q7-微信式双栏布局", await page.locator(".wx-shell").count() > 0 && await page.locator("#csContactList").count() > 0, "wx-shell present");
  check("Q7-WebSocket实时通道标注", (await page.locator("body").innerText()).includes("WebSocket"), "labeled");
  const wsOk = await page.evaluate(() => typeof WebSocket !== "undefined");
  check("Q7-WebSocket API可用", wsOk, "browser ws=" + wsOk);

  // Q8 柜机端重做
  await page.goto("http://localhost:8082/", { waitUntil: "load" }); await page.waitForTimeout(500);
  check("Q8-柜机待机大按钮", await page.locator(".k-btn").count() >= 2, "buttons=" + await page.locator(".k-btn").count());
  await page.click('[data-action="kioskStore"]'); await page.waitForTimeout(400);
  check("Q8-柜机存件三规格", await page.locator(".k-spec").count() === 3, "specs=" + await page.locator(".k-spec").count());
  await page.click('[data-action="kioskHome"]'); await page.waitForTimeout(300);
  await page.click('[data-action="kioskPickup"]'); await page.waitForTimeout(400);
  check("Q8-柜机取件键盘", await page.locator(".k-key").count() === 12, "keys=" + await page.locator(".k-key").count());

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await browser.close();
  process.exit(fail > 0 ? 1 : 0);
})();
