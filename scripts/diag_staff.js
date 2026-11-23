const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 1280, height: 860 } });
  p.on("pageerror", (e) => console.log("PAGEERROR:", e.message));
  await p.goto("http://localhost:8090/#m-login"); await p.waitForTimeout(300);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(700);
  await p.goto("http://localhost:8090/#m-staff"); await p.waitForTimeout(700);
  console.log("表格行数:", await p.locator(".tbl tbody tr").count());
  console.log("页面文本前200:", (await p.locator("#app").innerText()).slice(0, 200).replace(/\n/g, " / "));
  console.log("直接API:", await p.evaluate(async () => {
    const info = JSON.parse(localStorage.getItem("locker_admin"));
    const r = await fetch("/api/admin/staff", { headers: { "X-Token": info.token } });
    const d = await r.json();
    return Array.isArray(d) ? d.length : JSON.stringify(d);
  }));
  await b.close();
})();
