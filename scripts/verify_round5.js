/* 第 5 轮验证：复制成功提示 + 上传本地图片头像 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const fs = require("fs");
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (name, ok, detail) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + name + "  " + detail); };

// 生成一个测试用 PNG（1x1 红点，base64）
function makeTestPNG() {
  return Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==", "base64");
}

(async () => {
  const tmpPNG = path.join(__dirname, "..", "dist", "test-avatar.png");
  fs.writeFileSync(tmpPNG, makeTestPNG());

  const b = await chromium.launch({ headless: true });
  const ctx = await b.newContext({ viewport: { width: 1280, height: 900 }, permissions: ["clipboard-read", "clipboard-write"] });
  const p = await ctx.newPage();
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message));

  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.click('.login-tab[data-m="code"]'); await p.waitForTimeout(150); await p.fill("#phone", "13900001113"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(2200);

  // Q1 复制成功提示
  await p.goto(BASE + "#u-orders"); await p.waitForTimeout(500);
  await p.evaluate(() => { document.getElementById("toast-root").innerHTML = ""; });
  const code = await p.locator(".pickup-code-box b").first().innerText();
  const cp = p.locator(".copy-code").first();
  await cp.click(); await p.waitForTimeout(300);
  const toast = await p.locator("#toast-root").innerText();
  const btnText = await cp.innerText();
  check("Q1-复制成功toast", toast.includes("复制成功") && toast.includes(code), "toast=" + toast);
  check("Q1-按钮显示已复制", btnText.includes("已复制"), "btn=" + btnText);

  // Q2 上传本地图片头像
  await p.goto(BASE + "#u-mine"); await p.waitForTimeout(400);
  await p.click("#changeAvatar"); await p.waitForTimeout(200);
  check("Q2-上传按钮存在", await p.locator("#uploadAvatarBtn").count() > 0, "upload btn present");
  await p.locator("#avatarFile").setInputFiles(tmpPNG);
  await p.waitForTimeout(1200);
  const avatarHTML = await p.locator("#myAvatar").innerHTML();
  check("Q2-头像显示为图片", avatarHTML.includes("<img") && avatarHTML.includes("data:image/"), "html=" + avatarHTML.slice(0, 60));

  // 后端持久化验证
  const me = await p.evaluate(async () => {
    const tk = localStorage.getItem("locker_token");
    const r = await fetch("/api/user/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ phone: "13900001113", code: "1234", agree: true }) });
    return (await r.json()).avatar || "";
  });
  check("Q2-头像持久化到后端", me.startsWith("data:image/"), "avatar length=" + me.length);

  // 让当前用户发起客服会话（这样客服列表里才有该用户）
  await p.goto(BASE + "#u-chat"); await p.waitForTimeout(400);
  await p.fill("#chatInput", "测试头像同步"); await p.click("#chatSend"); await p.waitForTimeout(500);

  // 客服列表显示图片头像
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  await p.fill("#admId", "svc"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(500);
  await p.goto(BASE + "#m-cs"); await p.waitForTimeout(700);
  const avImgs = await p.locator(".wx-contact-list img.wx-avatar").count();
  check("Q2-客服列表图片头像", avImgs > 0, "img avatars=" + avImgs);

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  fs.unlinkSync(tmpPNG);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
