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

  // 登录用户A
  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900001113"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(900);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  await p.goto(BASE + "#u-home"); await p.waitForTimeout(900);
  const cabCardTxt = await p.locator("#cabStatusCard").innerText();
  check("Q1-无多余查看详情按钮", !cabCardTxt.includes("查看详情"), "card=" + cabCardTxt.slice(0, 50));
  const codes = await p.evaluate(() => [...document.querySelectorAll("#cabMini .cab-cell")].map((x) => x.dataset.code || ""));
  check("Q4-用户端柜格编号完整唯一", codes.length > 0 && /^[AB]-/.test(codes[0]) && new Set(codes).size === codes.length, "codes=" + codes.slice(0, 3).join(",") + " unique=" + new Set(codes).size + "/" + codes.length);
  // 点击格子看详情
  await p.locator("#cabMini .cab-cell").first().click(); await p.waitForTimeout(200);
  check("Q1-点格子看详情", (await p.locator("#toast-root").innerText()).includes("柜格"), "toast=" + (await p.locator("#toast-root").innerText()).slice(0, 40));

  // 地图标注+搜索
  await p.goto(BASE + "#u-outlets"); await p.waitForTimeout(3500);
  const mapTxt = await p.locator("#olMap").innerText();
  check("Q2-地图标注网点名", mapTxt.includes("人民广场") || mapTxt.includes("虹桥"), "map=" + mapTxt.slice(0, 60));
  check("Q2-地区搜索框", await p.locator("#olSearch").count() > 0 && await p.locator(".ol-city").count() >= 2, "cityBtns=" + await p.locator(".ol-city").count());
  await p.fill("#olSearch", "虹桥"); await p.waitForTimeout(600);
  check("Q2-搜索过滤列表", (await p.locator("#olList").innerText()).includes("虹桥"), "filtered");

  // 柜机端空闲数与编号
  await p.goto("http://localhost:8082/#k-standby"); await p.waitForTimeout(800);
  const kCenter = await p.locator(".k-center p").innerText();
  check("Q4-柜机显示本柜空闲数", kCenter.includes("本柜") && !/本柜\s*52/.test(kCenter), "text=" + kCenter);
  const kCodes = await p.evaluate(() => [...document.querySelectorAll(".k-art .d")].map((x) => x.textContent.trim()));
  check("Q4-柜机编号完整唯一", kCodes.length > 0 && /^[AB]-/.test(kCodes[0]) && new Set(kCodes).size === kCodes.length, "codes=" + kCodes.slice(0,3).join(",") + " unique=" + new Set(kCodes).size);

  // 后台网点切换
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  await p.fill("#admId", "site_rm"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(700);
  await p.goto(BASE + "#m-monitor"); await p.waitForTimeout(600);
  check("Q3-后台网点按钮带名称", (await p.locator(".mon-o").first().innerText()).includes("站") || (await p.locator(".mon-o").first().innerText()).includes("枢纽"), "btn=" + (await p.locator(".mon-o").first().innerText()).slice(0, 30));
  check("Q3-后台可切换多个网点", await p.locator(".mon-o").count() >= 2, "buttons=" + await p.locator(".mon-o").count());

  // 新用户默认头像（隔离）
  await p.goto(BASE + "#u-login"); await p.evaluate(() => localStorage.removeItem("locker_token")); await p.reload(); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150);
  await p.fill("#phone", "13900008888"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(900);
  if (await p.locator("#skipPwd").count()) { await p.click("#skipPwd"); await p.waitForTimeout(200); }
  await p.goto(BASE + "#u-mine"); await p.waitForTimeout(400);
  const av = await p.locator("#myAvatar").innerHTML();
  check("Q5-新用户默认头像非图片", !av.includes("<img") && av.includes("🙂"), "avatar=" + av.slice(0, 50));

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
