const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
(async () => {
  const b = await chromium.launch({ headless: true });
  const ctx = await b.newContext({ permissions: ["clipboard-read", "clipboard-write"] });
  const p = await ctx.newPage();
  await p.goto("http://localhost:8090/#u-login"); await p.waitForTimeout(300);
  await p.fill("#phone", "13900001113"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(2200);
  await p.goto("http://localhost:8090/#u-orders"); await p.waitForTimeout(500);
  await p.evaluate(() => { document.getElementById("toast-root").innerHTML = ""; });
  const code = await p.locator(".pickup-code-box b").first().innerText();
  await p.locator(".copy-code").first().click(); await p.waitForTimeout(500);
  const toast = await p.locator("#toast-root").innerText();
  let clip = "";
  try { clip = await p.evaluate(() => navigator.clipboard.readText()); } catch (e) { clip = "读取受限:" + e.message; }
  console.log("取件码=" + code + " toast=" + JSON.stringify(toast) + " 剪贴板=" + JSON.stringify(clip));
  console.log(toast.includes("已复制") || clip === code ? "[PASS] 复制功能生效" : "[FAIL] 复制未生效");
  await b.close();
})();
