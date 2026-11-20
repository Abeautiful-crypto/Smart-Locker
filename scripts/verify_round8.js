/* 第8轮：客服发送按钮尺寸 + 工单中文 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 1280, height: 860 } });
  await p.goto("http://localhost:8090/#m-login"); await p.waitForTimeout(300);
  await p.fill("#admId", "svc"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(600);
  await p.goto("http://localhost:8090/#m-cs"); await p.waitForTimeout(600);
  // 打开一个会话才有输入框
  if (await p.locator(".wx-contact").count()) { await p.locator(".wx-contact").first().click(); await p.waitForTimeout(500); }
  const sizes = await p.evaluate(() => {
    const btn = document.getElementById("wxSend");
    const inp = document.getElementById("wxInput");
    const br = btn ? btn.getBoundingClientRect() : null;
    const ir = inp ? inp.getBoundingClientRect() : null;
    return { btnW: br ? Math.round(br.width) : -1, btnH: br ? Math.round(br.height) : -1, inpW: ir ? Math.round(ir.width) : -1 };
  });
  const ok = sizes.btnW > 0 && sizes.btnW < 120 && sizes.btnH < 60 && sizes.inpW > 200;
  console.log((ok ? "[PASS]" : "[FAIL]") + " 发送按钮尺寸: " + JSON.stringify(sizes));
  const ticket = await p.locator("#ticketList").innerText();
  const clean = !/%[0-9A-Fa-f]{2}/.test(ticket);
  console.log((clean ? "[PASS]" : "[FAIL]") + " 工单中文无编码乱码");
  await p.screenshot({ path: "c:/Users/91526/Desktop/智能寄存柜/Smart-Locker/shots/fix-cs-send.png" });
  await b.close();
  process.exit(ok && clean ? 0 : 1);
})();
