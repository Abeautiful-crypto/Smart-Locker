/* 第11轮：库存联动/营收口径/柜格可视化/地图 */
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

  // 营收口径
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(300);
  await p.fill("#admId", "super"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(700);
  await p.goto(BASE + "#m-dash"); await p.waitForTimeout(400);
  const rev = await p.evaluate(async () => {
    const info = JSON.parse(localStorage.getItem("locker_admin"));
    return (await (await fetch("/api/admin/revenue?dim=outlet", { headers: { "X-Token": info.token } })).json());
  });
  check("Q2-净收益非负", rev.total >= 0, "income=" + rev.income + " refund=" + rev.refund + " total=" + rev.total);

  // 后台柜格监控（super）
  await p.goto(BASE + "#m-monitor"); await p.waitForTimeout(600);
  const cells = await p.locator(".cab-cell").count();
  check("Q3-后台柜格网格非空", cells > 0, "cells=" + cells);
  check("Q3-柜格有编号", (await p.locator(".cab-cell b").first().innerText().catch(() => "")) !== "", "number shown");
  check("Q3-状态图例存在", await p.locator(".cab-legend").count() > 0, "legend");

  // 网点管理员柜格监控（不空白）
  await p.goto(BASE + "#m-login"); await p.evaluate(() => localStorage.removeItem("locker_admin"));
  await p.reload(); await p.waitForTimeout(300);
  await p.fill("#admId", "site_rm"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(700);
  await p.goto(BASE + "#m-monitor"); await p.waitForTimeout(600);
  check("Q3-网点管理员监控非空", await p.locator(".cab-cell").count() > 0, "cells=" + await p.locator(".cab-cell").count());

  // 用户端库存联动 + 地图 + 柜格状态卡
  await p.goto(BASE + "#u-login"); await p.evaluate(() => localStorage.removeItem("locker_token")); await p.reload(); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900006666"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(900);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  await p.goto(BASE + "#u-home"); await p.waitForTimeout(900);
  check("Q3-用户端柜格状态卡", await p.locator("#cabMini .cab-cell").count() > 0, "cells=" + await p.locator("#cabMini .cab-cell").count());
  await p.goto(BASE + "#u-outlets"); await p.waitForTimeout(900);
  check("Q4-地图容器存在", await p.locator("#olMap").count() > 0, "map present");
  const mapTxt = await p.locator("#olMap").innerText();
  check("Q4-地图或兜底可见", mapTxt.length > 0 || await p.locator(".leaflet-container").count() > 0, "map rendering");

  // 柜机端柜格状态
  await p.goto("http://localhost:8082/#k-standby"); await p.waitForTimeout(700);
  check("Q3-柜机端柜格状态", await p.locator(".k-art .d").count() > 0, "cells=" + await p.locator(".k-art .d").count());

  // 0格禁止下单
  const zero = await p.evaluate(async () => {
    const d = await (await fetch("/api/outlets/1")).json();
    const zeroSize = Object.entries(d.idleBySize || {}).find(([, n]) => n === 0);
    return zeroSize ? zeroSize[0] : null;
  });
  if (zero) {
    await p.goto(BASE + "#u-spec"); await p.waitForTimeout(500);
    const btn = p.locator('.spec-card[data-size="' + zero + '"]');
    check("Q1-0格柜型禁用", await btn.count() === 0 || (await btn.getAttribute("data-empty")) === "1", "size=" + zero);
  } else check("Q1-0格柜型禁用", true, "当前无0格规格，跳过");

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
