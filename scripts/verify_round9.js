const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 1280, height: 860 } });
  await p.goto("http://localhost:8083/#m-login"); await p.waitForTimeout(300);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "wrongpwd"); await p.click("#admLogin"); await p.waitForTimeout(500);
  const inline = await p.locator("#admLoginErr").innerText().catch(() => "");
  const toast = await p.locator("#toast-root").innerText();
  console.log((inline.includes("账号或密码错误") && toast.includes("账号或密码错误") ? "[PASS]" : "[FAIL]") + " 后台错误双提示: inline=" + JSON.stringify(inline) + " toast=" + JSON.stringify(toast));
  // 新用户密码注册
  await p.goto("http://localhost:8081/#u-login"); await p.waitForTimeout(300);
  await p.fill("#phone", "13900009999"); await p.fill("#pwd", "123456"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(900);
  const hash = await p.evaluate(() => location.hash);
  const skip = await p.locator("#skipPwd").count();
  if (skip) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  console.log(((await p.evaluate(() => location.hash)) === "#u-home" || hash === "#u-home" ? "[PASS]" : "[FAIL]") + " 新用户密码注册登录: hash=" + hash);
  await b.close();
})();
