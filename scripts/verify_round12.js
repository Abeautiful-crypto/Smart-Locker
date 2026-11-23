const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (n, ok, d) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + n + "  " + d); };

(async () => {
  const b = await chromium.launch({ headless: true });
  const p = await b.newPage({ viewport: { width: 1280, height: 900 } });
  const errs = []; p.on("pageerror", (e) => errs.push(e.message));

  // 用户登录 → 首页柜机状态卡
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900007777"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(900);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  await p.goto(BASE + "#u-home"); await p.waitForTimeout(900);
  const loc = await p.locator("#cabLoc").innerText();
  check("Q1-柜机状态显示网点位置", loc.length > 3 && !loc.includes("undefined"), "loc=" + loc);
  check("Q4-可切换网点", await p.locator("#cabOutletSel option").count() >= 2, "options=" + await p.locator("#cabOutletSel option").count());
  // 切换网点
  await p.selectOption("#cabOutletSel", "2"); await p.waitForTimeout(500);
  check("Q4-切换后位置更新", (await p.locator("#cabLoc").innerText()).includes("虹桥"), "loc=" + await p.locator("#cabLoc").innerText());
  // 详情弹窗不跳地图
  await p.locator("#cabStatusCard").click({ position: { x: 20, y: 20 } }); await p.waitForTimeout(300);
  check("Q2-详情弹窗非地图", await p.locator(".cab-grid").count() > 0 || (await p.evaluate(() => location.hash)) === "#u-home", "hash=" + await p.evaluate(() => location.hash));
  const modalTxt = await p.locator("#modal-root").innerText();
  check("Q2-详情含网点地址", modalTxt.includes("虹桥") || modalTxt.includes("上海"), "modal=" + modalTxt.slice(0, 60));
  if (await p.locator("#cabOk").count()) await p.click("#cabOk");

  // 地图兜底（离线示意图可见+带网点名）
  await p.goto(BASE + "#u-outlets"); await p.waitForTimeout(3500);
  const mapTxt = await p.locator("#olMap").innerText();
  check("Q3-地图可见且非空白", mapTxt.length > 5, "mapText=" + mapTxt.slice(0, 60));

  // 柜机端状态与用户端一致 + 图例
  await p.goto("http://localhost:8082/#k-standby"); await p.waitForTimeout(800);
  const kTxt = await p.locator(".k-art").innerText();
  check("Q3-柜机端有状态图例", kTxt.includes("空闲") && kTxt.includes("占用") && kTxt.includes("异常"), "legend=" + kTxt.replace(/\n/g, " "));
  check("Q1-柜机端显示网点名", kTxt.includes("人民广场") || kTxt.includes("虹桥"), "kiosk=" + kTxt.slice(0, 40));
  const kErr = await p.evaluate(() => ({
    err: document.querySelectorAll(".k-art .d.err").length,
    used: document.querySelectorAll(".k-art .d.used").length,
    free: document.querySelectorAll(".k-art .d").length,
  }));
  check("Q3-柜机端非全异常", kErr.err < kErr.free, JSON.stringify(kErr));

  // 后台订单详情
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(700);
  await p.goto(BASE + "#m-orders"); await p.waitForTimeout(600);
  check("Q6-订单详情按钮", await p.locator(".ord-detail").count() > 0, "buttons=" + await p.locator(".ord-detail").count());
  const detailTxt = await p.locator(".tbl").innerText();
  check("Q5-费用口径说明", detailTxt.includes("预付") && (await p.locator("#app").innerText()).includes("免费时段"), "prepaid column present");
  await p.locator(".ord-detail").first().click(); await p.waitForTimeout(300);
  check("Q6-详情弹窗有数据", (await p.locator("#modal-root").innerText()).includes("订单详情"), "modal shown");

  // 后台监控网点名
  await p.goto(BASE + "#m-monitor"); await p.waitForTimeout(500);
  check("Q4-后台监控柜点名", (await p.locator("#app").innerText()).includes("人民广场") || (await p.locator("#app").innerText()).includes("虹桥"), "outlet name shown");

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
