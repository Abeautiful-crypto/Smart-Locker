/* 领券功能验证：发布→用户可见→领取→库存扣减→每人限一张→库存0停售 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (n, ok, d) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + n + "  " + d); };

(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 430, height: 860 } });

  // 后台发布一张 totalCount=1 的券
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(300);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(600);
  await p.goto(BASE + "#m-coupons"); await p.waitForTimeout(400);
  await p.click("#addCp"); await p.waitForTimeout(200);
  await p.fill("#cpName", "验证限量券");
  await p.selectOption("#cpType", "amount");
  await p.fill("#cpVal", "2");
  await p.fill("#cpTotal", "1");
  await p.click("#cpSave"); await p.waitForTimeout(500);
  const adminText = await p.locator("table").innerText();
  const defId = (adminText.match(/验证限量券/) && await p.evaluate(async () => {
    const info = JSON.parse(localStorage.getItem("locker_admin"));
    const rs = await fetch("/api/admin/coupons", { headers: { "X-Token": info.token } });
    return (await rs.json()).find((d) => d.name === "验证限量券").id;
  }));
  check("后台发布限量券", !!defId, "defId=" + defId);

  // 用户A登录，查看领券中心
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900003331"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(1000);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  await p.goto(BASE + "#u-coupons"); await p.waitForTimeout(500);
  const haveClaim = await p.locator(".claim-btn").count();
  check("用户看到领券中心", haveClaim > 0, "claimBtns=" + haveClaim);

  // 领取
  const myClaim = p.locator(".claim-btn").filter({ hasText: "领取" }).first();
  if (await myClaim.count()) await myClaim.click();
  await p.waitForTimeout(500);
  const usable = await p.locator(".section-h").allInnerTexts();
  const couponsAfter = await p.evaluate(async () => {
    const tk = localStorage.getItem("locker_token");
    const r = await fetch("/api/user/coupons", { headers: { "X-Token": tk } });
    return await r.json();
  });
  check("领取后进入可使用", (couponsAfter.usable || []).some((c) => c.name === "验证限量券"), "usable=" + couponsAfter.usable.length);

  // 重复领取应失败
  const dup = await p.evaluate(async (id) => {
    const tk = localStorage.getItem("locker_token");
    const r = await fetch("/api/user/coupons/" + id + "/claim", { method: "POST", headers: { "Content-Type": "application/json", "X-Token": tk }, body: "{}" });
    return r.status;
  }, defId);
  check("每人限领一张", dup === 400, "重复领取status=" + dup);

  // 库存为0：另一用户看到"已抢完"
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(200);
  await p.evaluate(() => { localStorage.removeItem("locker_token"); });
  await p.reload(); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900003332"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(1000);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  await p.goto(BASE + "#u-coupons"); await p.waitForTimeout(500);
  const avail = await p.evaluate(async () => {
    const tk = localStorage.getItem("locker_token");
    const r = await fetch("/api/user/coupons/available", { headers: { "X-Token": tk } });
    return (await r.json()).coupons;
  });
  const target = avail.find((c) => c.name === "验证限量券");
  check("库存0停售", target && target.left === 0 && target.soldOut === true, "target=" + JSON.stringify(target));

  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
