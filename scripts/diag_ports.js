/* 三端口端标识与布局验证 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));

const cases = [
  { name: "user-8081", port: 8081, end: "u", vp: { width: 430, height: 860 }, hash: "" },
  { name: "kiosk-8082", port: 8082, end: "k", vp: { width: 1200, height: 800 }, hash: "" },
  { name: "admin-8083", port: 8083, end: "m", vp: { width: 1280, height: 860 }, hash: "" },
];

(async () => {
  const browser = await chromium.launch({ headless: true });
  const errors = [];
  let ok = 0, bad = 0;
  for (const c of cases) {
    const page = await browser.newPage({ viewport: c.vp });
    page.on("pageerror", (e) => errors.push(c.name + " pageerror: " + e.message));
    page.on("console", (m) => { if (m.type() === "error") errors.push(c.name + " console: " + m.text()); });
    await page.goto("http://localhost:" + c.port + "/", { waitUntil: "load" });
    await page.waitForTimeout(900);
    const info = await page.evaluate(() => ({
      end: window.LOCKER_END,
      hash: location.hash,
      appClass: document.getElementById("app").className,
      width: Math.round(document.getElementById("app").getBoundingClientRect().width),
    }));
    const good = info.end === c.end && info.appClass.includes("end-" + ({ u: "user", k: "kiosk", m: "admin" })[c.end]);
    good ? ok++ : bad++;
    console.log((good ? "[PASS] " : "[FAIL] ") + c.name + " end=" + info.end + " hash=" + info.hash + " appClass=" + info.appClass + " appWidth=" + info.width);
    await page.screenshot({ path: "c:/Users/91526/Desktop/智能寄存柜/Smart-Locker/shots/diag-" + c.name + ".png" });
    await page.close();
  }
  console.log("JS错误:", errors.length ? errors.join("\n") : "无");
  console.log("RESULT PASS=" + ok + " FAIL=" + bad);
  await browser.close();
  process.exit(bad > 0 ? 1 : 0);
})();
