const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (n, ok, d) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + n + "  " + d); };

(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 430, height: 860 } });
  const errs = []; p.on("pageerror", (e) => errs.push(e.message));

  // 登录页无提示块
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  const loginTxt = await p.locator("#app").innerText();
  check("Q3-登录页无默认密码提示", !loginTxt.includes("初始密码"), "clean");
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  const adminTxt = await p.locator("#app").innerText();
  check("Q3-后台登录无演示账号提示", !adminTxt.includes("演示账号"), "clean");

  // 用户登录 → 下单 → 存件流程
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(200);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900005555"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(900);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  const tk = await p.evaluate(() => localStorage.getItem("locker_token"));
  const order = await p.evaluate(async (tk) => {
    const r = await fetch("/api/orders", { method: "POST", headers: { "Content-Type": "application/json", "X-Token": tk }, body: JSON.stringify({ outletId: 2, size: "M", payMode: "prepaid" }) });
    return await r.json();
  }, tk);
  check("下单成功", !!order.code, "code=" + order.code);
  // 直接进 u-open 需 lastOrder，用真实 UI 下单路径不便，此处用 evaluate 注入
  await p.evaluate((o) => { /* 无法直接改模块内 state，走 UI */ }, order);
  // 通过 UI 下单：首页→网点→规格→支付
  await p.goto(BASE + "#u-home"); await p.waitForTimeout(300);
  await p.goto(BASE + "#u-outlets"); await p.waitForTimeout(400);
  await p.locator('[data-action="outlet"]').nth(1).click(); await p.waitForTimeout(400);
  await p.goto(BASE + "#u-spec"); await p.waitForTimeout(400);
  await p.click('[data-size="M"]'); await p.waitForTimeout(200);
  await p.click("#toPay"); await p.waitForTimeout(400);
  await p.click("#doPay"); await p.waitForTimeout(500);
  check("Q2-存件步骤0支付成功", (await p.locator(".step").count()) === 5 && (await p.locator("#openGo").count()) > 0, "steps=" + await p.locator(".step").count());
  await p.click("#openGo"); await p.waitForTimeout(300);
  check("Q2-开柜动画中", await p.locator(".locker-door.opening, .locker-door.open").count() > 0, "door animating");
  await p.waitForTimeout(1200);
  check("Q2-柜门已开可放入", await p.locator("#closeGo").count() > 0, "close button present");
  await p.click("#closeGo"); await p.waitForTimeout(300);
  check("Q2-关柜动画中", await p.locator(".locker-door.closing, .locker-door.closed").count() > 0, "closing");
  await p.waitForTimeout(1200);
  check("Q2-存件完成", (await p.locator("#app").innerText()).includes("存件完成"), "done");

  // 员工列表
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(600);
  await p.goto(BASE + "#m-staff"); await p.waitForTimeout(500);
  check("Q1-员工列表3行", await p.locator(".tbl tbody tr").count() === 3, "rows=" + await p.locator(".tbl tbody tr").count());
  check("Q3-员工页无说明卡", !(await p.locator("#app").innerText()).includes("登录态立即失效"), "clean");

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
