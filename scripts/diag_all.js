/* 全量页面巡检：三端所有路由 + 登录页控件尺寸 + JS 错误 + 横向溢出 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";

// u-open 需下单后的 state.lastOrder（无订单会跳回首页），由 e2e_check.js 全流程覆盖
const U = ["u-home", "u-outlets", "u-spec", "u-pay", "u-orders", "u-detail", "u-pickup", "u-coupons", "u-coupon-select", "u-chat", "u-guide", "u-mine"];
const K = ["k-standby", "k-store", "k-pickup"];
const M = ["m-dash", "m-outlets", "m-monitor", "m-rules", "m-coupons", "m-orders", "m-revenue", "m-alerts", "m-staff", "m-cs"];

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 } });
  const errors = [];
  page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
  page.on("response", (r) => { if (r.status() >= 400) errors.push("HTTP " + r.status() + " " + r.url()); });
  page.on("console", (m) => { if (m.type() === "error") errors.push("console: " + m.text()); });

  // ---- 登录页控件尺寸检查（用户反馈的验证码框塌陷） ----
  await page.goto(BASE + "#u-login", { waitUntil: "load" });
  await page.waitForTimeout(500);
  const login = await page.evaluate(() => {
    const box = (id) => { const e = document.getElementById(id); return e ? Math.round(e.getBoundingClientRect().width) : -1; };
    return { codeW: box("code"), phoneW: box("phone"), sendW: box("sendCode"), appClass: document.getElementById("app").className };
  });
  const loginOK = login.codeW >= 100 && login.phoneW >= 100 && login.sendW >= 40 && login.appClass === "end-user";
  console.log((loginOK ? "[PASS]" : "[FAIL]") + " u-login 控件宽度: " + JSON.stringify(login));
  await page.screenshot({ path: "c:/Users/91526/Desktop/智能寄存柜/Smart-Locker/shots/fix-u-login.png" });

  // ---- C 端登录，带 token 巡检 u-* ----
  await page.fill("#phone", "13900001113");
  await page.fill("#code", "1234");
  await page.check("#agreeChk");
  await page.click("#doLogin");
  await page.waitForTimeout(700);

  let fail = loginOK ? 0 : 1;
  async function inspect(list, label) {
    for (const v of list) {
      await page.goto(BASE + "#" + v, { waitUntil: "load" });
      await page.waitForTimeout(320);
      const r = await page.evaluate(() => {
        const app = document.getElementById("app");
        const txt = app.innerText.trim();
        return {
          hash: location.hash, cls: app.className, len: txt.length,
          head: txt.slice(0, 40).replace(/\n+/g, " "),
          overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 2,
        };
      });
      const good = r.len > 10 && !r.overflow && r.hash === "#" + v;
      if (!good) fail++;
      console.log((good ? "[PASS]" : "[FAIL]") + " " + label + "/" + v + " len=" + r.len + " cls=" + r.cls + " overflow=" + r.overflow + " :: " + r.head);
    }
  }
  await inspect(U, "U");
  await inspect(K, "K");

  // ---- 后台登录，带 token 巡检 m-* ----
  await page.goto(BASE + "#m-login", { waitUntil: "load" });
  await page.waitForTimeout(400);
  await page.fill("#admId", "super");
  await page.fill("#admPwd", "000000");
  await page.click("#admLogin");
  await page.waitForTimeout(700);
  await inspect(M, "M");

  console.log("JS错误:", errors.length ? errors.join("\n") : "无");
  console.log("RESULT FAIL=" + fail + " JSERR=" + errors.length);
  await browser.close();
  process.exit(fail > 0 || errors.length > 0 ? 1 : 0);
})();
