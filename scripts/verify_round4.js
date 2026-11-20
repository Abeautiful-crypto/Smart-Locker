/* 第 4 轮用户反馈验证：复制/价格/头像/聊天气泡 */
const path = require("path");
const pwDir = "C:/Users/91526/.ai-manager/runtimes/node/24.21.0/node_modules/@playwright/cli/node_modules/playwright-core";
const { chromium } = require(path.join(pwDir, "index.js"));
const BASE = "http://localhost:8090/";
let pass = 0, fail = 0;
const check = (name, ok, detail) => { ok ? pass++ : fail++; console.log((ok ? "[PASS] " : "[FAIL] ") + name + "  " + detail); };

(async () => {
  const b = await chromium.launch({ headless: true });
  const ctx = await b.newContext({ viewport: { width: 1280, height: 900 }, permissions: ["clipboard-read", "clipboard-write"] });
  const p = await ctx.newPage();
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message));

  await p.goto(BASE + "#u-login"); await p.waitForTimeout(300);
  await p.fill("#phone", "13900001113"); await p.fill("#code", "1234"); await p.check("#agreeChk"); await p.click("#doLogin"); await p.waitForTimeout(500);

  // Q1 复制按钮
  await p.goto(BASE + "#u-orders"); await p.waitForTimeout(400);
  const cp = p.locator(".copy-code").first();
  if (await cp.count()) {
    const code = await p.locator(".pickup-code-box b").first().innerText();
    await cp.click(); await p.waitForTimeout(400);
    const toastTxt = await p.locator("#toast-root").innerText();
    check("Q1-复制按钮反馈", toastTxt.includes("复制") || toastTxt.includes(code), "toast=" + toastTxt + " code=" + code);
  } else check("Q1-复制按钮", false, "无复制按钮");

  // Q2 柜型价格
  await p.goto(BASE + "#u-spec"); await p.waitForTimeout(500);
  const priceText = await p.locator(".spec-card").first().innerText();
  const prices = await p.locator(".spec-card .mono").allInnerTexts();
  check("Q2-柜型价格非0", !prices.every((x) => x.includes("0.00")), "prices=" + JSON.stringify(prices));

  // Q3 头像切换
  await p.goto(BASE + "#u-mine"); await p.waitForTimeout(400);
  check("Q3-我的页有头像", await p.locator("#myAvatar").count() > 0, "avatar=" + await p.locator("#myAvatar").innerText().catch(() => ""));
  await p.click("#changeAvatar"); await p.waitForTimeout(200);
  check("Q3-头像选择器", await p.locator(".avatar-opt").count() === 8, "opts=" + await p.locator(".avatar-opt").count());
  await p.locator('.avatar-opt[data-a="🐼"]').click(); await p.waitForTimeout(400);
  check("Q3-头像切换生效", (await p.locator("#myAvatar").innerText()).includes("🐼"), "avatar=" + await p.locator("#myAvatar").innerText());

  // 客服端：用户列表头像 + 聊天气泡方向
  await p.goto(BASE + "#m-login"); await p.waitForTimeout(200);
  await p.fill("#admId", "svc"); await p.fill("#admPwd", "000000"); await p.click("#admLogin"); await p.waitForTimeout(500);
  await p.goto(BASE + "#m-cs"); await p.waitForTimeout(600);
  const contacts = await p.locator(".wx-contact").count();
  if (contacts) {
    const avTxt = await p.locator(".wx-avatar").first().innerText();
    check("Q3-客服列表用户头像", !!avTxt && avTxt.length > 0, "avatar=" + avTxt);
    await p.locator(".wx-contact").first().click(); await p.waitForTimeout(600);
    const bubbles = await p.evaluate(() => {
      const all = [...document.querySelectorAll("#wxMsgs .chat-bubble")];
      return all.map((el) => ({ cls: el.className, text: el.innerText.slice(0, 12) }));
    });
    const userOnLeft = bubbles.some((x) => x.cls.includes("other") && x.text.includes("用户"));
    const agentOnRight = bubbles.some((x) => x.cls.includes("me") && (x.text.includes("我") || x.text.includes("人工")));
    check("Q4-客服端用户消息在左", userOnLeft, "bubbles=" + JSON.stringify(bubbles.slice(0, 4)));
    check("Q4-客服端自己消息在右", agentOnRight || bubbles.length === 0, "agentRight=" + agentOnRight);
  } else {
    check("Q3-客服列表用户头像", false, "无联系人");
  }

  console.log("JS错误:", errs.length ? errs.join("\n") : "无");
  console.log("RESULT PASS=" + pass + " FAIL=" + fail);
  await b.close();
  process.exit(fail > 0 ? 1 : 0);
})();
