const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 1280, height: 860 } });
  p.on("pageerror", (e) => console.log("PAGEERROR:", e.message));
  p.on("response", (r) => { if (r.url().includes("/login")) console.log("HTTP", r.status(), r.url()); });

  // 后台登录：错误密码
  await p.goto("http://localhost:8083/#m-login"); await p.waitForTimeout(400);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "wrongpass"); await p.click("#admLogin"); await p.waitForTimeout(600);
  console.log("后台错误密码 toast:", JSON.stringify(await p.locator("#toast-root").innerText()));
  console.log("后台错误密码 hash:", await p.evaluate(() => location.hash));

  // 用户端密码登录：新手机号 + 任意密码
  await p.goto("http://localhost:8081/#u-login"); await p.waitForTimeout(300);
  await p.fill("#phone", "13900002223"); await p.fill("#pwd", "123456"); await p.check("#agreeChk");
  await p.click("#doLogin"); await p.waitForTimeout(600);
  console.log("新用户密码登录 toast:", JSON.stringify(await p.locator("#toast-root").innerText()));
  console.log("新用户密码登录 hash:", await p.evaluate(() => location.hash));
  await b.close();
})();
