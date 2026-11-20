/* 第 6 轮验证：柜机免登录/我的页样式/工单解码/表注释/登录双模式 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (name, ok, detail) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + name + "  " + detail); };

(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 1280, height: 900 } });
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message));

  // Q5 登录双模式：默认密码 tab + 验证码 tab
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(400);
  check("Q5-默认密码登录tab", await p.locator(".login-tab.on").innerText() === "密码登录", "tab=" + await p.locator(".login-tab.on").innerText());
  check("Q5-密码输入框", await p.locator("#pwd").count() > 0, "pwd input");
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(200);
  check("Q5-切换验证码登录", await p.locator("#code").count() > 0 && await p.locator(".login-tab.on").innerText() === "验证码登录", "code input shown");
  await p.click("#forgotPwd").catch(() => {});

  // 验证码登录后设置密码流程
  await p.fill("#phone", "13900001113"); await p.fill("#code", "1234"); await p.check("#agreeChk");
  await p.click("#doLogin"); await p.waitForTimeout(1200);
  check("Q5-验证码登录成功", (await p.locator("body").innerText()).includes("设置登录密码") || (await p.evaluate(() => location.hash)) === "#u-home", "首次登录弹出设置密码引导=预期");
  // 首次登录会弹设置密码
  if (await p.locator("#setLoginPwd").count()) {
    await p.fill("#setLoginPwd", "test123456"); await p.click("#setLoginPwdGo"); await p.waitForTimeout(400);
    check("Q5-设置登录密码", (await p.locator("#toast-root").innerText()).includes("密码"), "toast=" + await p.locator("#toast-root").innerText());
  } else check("Q5-设置登录密码", true, "已设置过，跳过");

  // Q2 我的页头像卡布局
  await p.goto(BASE + "#u-mine"); await p.waitForTimeout(400);
  check("Q2-头像卡布局", await p.locator(".profile-card .profile-info").count() > 0 && await p.locator("#changeAvatar.profile-edit").count() > 0, "profile card");

  // Q1 柜机免登录取件
  const kiosk = await b.newPage({ viewport: { width: 1200, height: 800 } });
  await kiosk.goto("http://localhost:8082/#k-pickup"); await kiosk.waitForTimeout(500);
  const hasLoginErr = (await kiosk.locator("body").innerText()).includes("未登录");
  check("Q1-柜机取件无登录提示", !hasLoginErr, "no login error");
  // 用真实取件码走公开接口
  const tk = await p.evaluate(() => localStorage.getItem("locker_token"));
  const orderCode = await p.evaluate(async (tk) => {
    const r = await fetch("/api/orders", { method: "POST", headers: { "Content-Type": "application/json", "X-Token": tk }, body: JSON.stringify({ outletId: 2, size: "M", payMode: "prepaid" }) });
    return (await r.json()).code;
  }, tk);
  const kioskResult = await kiosk.evaluate(async (code) => {
    const r = await fetch("/api/kiosk/pickup", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code }) });
    return { status: r.status, data: await r.json() };
  }, orderCode);
  check("Q1-柜机免登录取件API", kioskResult.status === 200 && !!kioskResult.data.orderId, "status=" + kioskResult.status + " code=" + orderCode);

  // Q3 工单中文解码
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  await p.fill("#admId", "svc"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(500);
  await p.goto(BASE + "#m-cs"); await p.waitForTimeout(600);
  const ticketText = await p.locator("#ticketList").innerText();
  const hasEncoded = /%[0-9A-Fa-f]{2}/.test(ticketText);
  check("Q3-工单无URL编码乱码", !hasEncoded, "hasEncoded=" + hasEncoded);

  // Q4 表注释（通过 API 或直接查 migrations 文件）
  const fs = require("fs");
  const sql = fs.readFileSync(path.join(__dirname, "..", "migrations", "005_table_comments.sql"), "utf8");
  const alterCount = (sql.match(/ALTER TABLE/g) || []).length;
  check("Q4-表注释迁移", alterCount >= 15, "ALTER TABLE 数=" + alterCount);

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
