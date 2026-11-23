/* Smart-Locker 前端 SPA：用户端 / 柜机端 / 管理后台（vanilla JS，无框架） */
"use strict";

/* ============ 基础工具 ============ */
const $ = (sel) => document.querySelector(sel);
const esc = (s) => String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const yuan = (fen) => ((fen || 0) / 100).toFixed(2).replace(/\.00$/, "");
const yuan2 = (fen) => ((fen || 0) / 100).toFixed(2);

const I = {
  back: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path d="M15 5l-7 7 7 7"/></svg>',
  pin: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 21s-7-6.2-7-11a7 7 0 1 1 14 0c0 4.8-7 11-7 11z"/><circle cx="12" cy="10" r="2.6"/></svg>',
  box: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="6" width="18" height="14" rx="2"/><path d="M3 10h18M12 6v-3M9 6h6"/></svg>',
  key: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="8" cy="8" r="4"/><path d="M11 11l9 9M17 17l3-3"/></svg>',
  list: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/></svg>',
  user: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="8" r="4"/><path d="M4 21c0-4 3.6-6 8-6s8 2 8 6"/></svg>',
  chat: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a8 8 0 0 1-8 8H4l2-3a8 8 0 1 1 15-5z"/></svg>',
  ticket: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 9V6a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v3a2 2 0 0 0 0 6v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-3a2 2 0 0 0 0-6z"/></svg>',
};

/* ============ API 客户端 ============ */
async function api(path, method, body) {
  const headers = { "Content-Type": "application/json" };
  const tk = localStorage.getItem("locker_token");
  if (tk) headers["X-Token"] = tk;
  const resp = await fetch(path, { method: method || "GET", headers, body: body ? JSON.stringify(body) : undefined });
  let data = null;
  try { data = await resp.json(); } catch (e) { /* 空响应 */ }
  if (!resp.ok) throw new Error((data && data.error) || ("HTTP " + resp.status));
  return data;
}

/* ============ 状态 ============ */
const state = {
  city: "上海",
  outlet: null,       // 当前网点详情
  spec: null,         // 选中的柜型
  payMode: "prepaid",
  couponId: 0,
  quote: null,
  lastOrder: null,    // 下单结果
  pickupCodeInput: "",
  adminToken: null,
  adminInfo: null,
  chatAfter: 0,
  chatTimer: null,
  avatar: localStorage.getItem("locker_avatar") || "🙂",
};
const AVATARS = ["🙂", "😎", "🐶", "🐱", "🦊", "🐼", "🚀", "🌟"];

// 剪贴板：优先 Clipboard API，失败回退 execCommand（IDE 内嵌浏览器/非安全上下文可用）
async function copyText(text) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch (e) {}
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.cssText = "position:fixed;left:-9999px;top:0";
    document.body.appendChild(ta);
    ta.focus(); ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch (e) { return false; }
}
// 柜格实时状态网格（后台监控/柜机端/用户端共用）
function cabinetGrid(cabinets, showNo) {
  if (!cabinets || !cabinets.length) return '<div class="faint tiny">暂无柜格数据</div>';
  const cls = { idle: "free", occupied: "used", fault: "err" };
  const txt = { idle: "空闲", occupied: "占用", fault: "异常" };
  return '<div class="cab-grid">' + cabinets.map((c) =>
    '<div class="cab-cell ' + (cls[c.status] || "free") + '" data-lid="' + (c.id || "") + '" title="' + esc(c.code) + ' ' + (txt[c.status] || "") + '">' +
    (showNo ? '<b>' + esc(c.code.replace(/^[A-Za-z]-?/, "")) + '</b><span>' + (txt[c.status] || "") + '</span>' : "") + '</div>').join("") + '</div>';
}
function cabinetLegend(byStatus) {
  const s = byStatus || {};
  return '<div class="cab-legend"><span><i class="free"></i>空闲 ' + (s.idle || 0) + '</span><span><i class="used"></i>占用 ' + (s.occupied || 0) + '</span><span><i class="err"></i>异常 ' + (s.fault || 0) + '</span></div>';
}

// 头像渲染：图片 data URL 用 <img>，否则按 emoji/字符显示
function avatarHTML(av, cls) {
  const a = av || "🙂";
  const c = cls || "wx-avatar";
  const attr = ' data-avatar="' + esc(a) + '"';
  return a.indexOf("data:image/") === 0
    ? '<img class="' + c + '" src="' + a + '" alt="头像"' + attr + '>'
    : '<div class="' + c + '"' + attr + '>' + esc(a) + '</div>';
}
// 本地图片压缩为 128x128 JPEG data URL（控制体积，便于存库与传输）
function imageToAvatar(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("读取图片失败"));
    reader.onload = () => {
      const img = new Image();
      img.onerror = () => reject(new Error("图片解析失败"));
      img.onload = () => {
        const canvas = document.createElement("canvas");
        canvas.width = 128; canvas.height = 128;
        const ctx = canvas.getContext("2d");
        const side = Math.min(img.width, img.height);
        ctx.drawImage(img, (img.width - side) / 2, (img.height - side) / 2, side, side, 0, 0, 128, 128);
        resolve(canvas.toDataURL("image/jpeg", 0.82));
      };
      img.src = reader.result;
    };
    reader.readAsDataURL(file);
  });
}
function saveToken(tk) { localStorage.setItem("locker_token", tk || ""); }
function hasToken() { return !!localStorage.getItem("locker_token"); }

/* ============ Toast / Modal ============ */
function toast(msg, ms) {
  const root = $("#toast-root");
  root.innerHTML = ""; // 新提示覆盖旧的，避免多条堆叠看不清
  const el = document.createElement("div");
  el.className = "toast";
  el.textContent = msg;
  root.appendChild(el);
  setTimeout(() => { if (el.isConnected) el.remove(); }, ms || 2600);
}
function modal(html) {
  const root = $("#modal-root");
  root.innerHTML = '<div class="modal-mask"><div class="modal-box">' + html + '</div></div>';
  root.querySelector(".modal-mask").addEventListener("click", (e) => {
    if (e.target.classList.contains("modal-mask")) closeModal();
  });
}
function closeModal() { $("#modal-root").innerHTML = ""; }
function confirmBox(msg, onOk) {
  modal('<div class="bold14">确认操作</div><div class="mt10 faint">' + esc(msg) + '</div>' +
    '<div class="row gap10 mt16"><button class="btn flex1" id="mCancel">取消</button><button class="btn btn-primary flex1" id="mOk">确认</button></div>');
  $("#mCancel").onclick = closeModal;
  $("#mOk").onclick = () => { closeModal(); onOk(); };
}

/* ============ 端标识（由后端按端口注入 index.html） ============ */
const LOCKER_END = (window.LOCKER_END && window.LOCKER_END !== "__LOCKER_END__") ? window.LOCKER_END : "all";
const END_DEFAULT = { u: "u-home", k: "k-standby", m: "m-login", all: "u-home" };
const END_PREFIX = { u: "u-", k: "k-", m: "m-" };
function defaultView() { return END_DEFAULT[LOCKER_END] || "u-home"; }
function viewEnd(v) { return v.startsWith("k-") ? "kiosk" : v.startsWith("m-") ? "admin" : "user"; }
function endAllowed(v) { return LOCKER_END === "all" || v.startsWith(END_PREFIX[LOCKER_END] || ""); }

/* ============ 路由 ============ */
const routes = {};
function go(view) { location.hash = "#" + view; }
function currentView() { return (location.hash || "#" + defaultView()).slice(1); }
window.addEventListener("hashchange", render);

function render() {
  stopChatPoll();
  if (state._beforeRender) { state._beforeRender(); state._beforeRender = null; }
  let v = currentView();
  // 独立端口只允许访问本端页面，跨端 hash 自动回到本端首页
  if (!endAllowed(v)) {
    location.replace("#" + defaultView());
    return;
  }
  const fn = routes[v] || routes[defaultView()];
  state._after = null;
  Promise.resolve(fn()).then((html) => {
    if (currentView() !== v) return;
    $("#app").className = "end-" + viewEnd(v);
    $("#app").innerHTML = html || "";
    $("#app").dataset.view = v;
    window.scrollTo(0, 0);
    if (state._after) { const f = state._after; state._after = null; f(); }
    if (v === "u-chat") startChatPoll();
  }).catch((err) => {
    if (currentView() === v) $("#app").innerHTML = '<div class="app-pad mt20"><div class="card">' + esc(err.message) + "</div></div>";
    toast(err.message);
  });
}

/* 事件委托：data-action */
document.addEventListener("click", (e) => {
  const el = e.target.closest("[data-action]");
  if (!el) return;
  const act = el.dataset.action;
  const id = el.dataset.id;
  const acts = {
    nav: () => go(id),
    back: () => history.back(),
    city: () => openCityPicker(),
    outlet: () => openOutletDetail(Number(id)),
    spec: () => { if (!requireLogin()) return; go("u-spec"); },
    paymode: () => { state.payMode = id; render(); },
    coupon: () => chooseCoupon(Number(id) || 0),
    couponSelect: () => go("u-coupon-select"),
    report: () => reportProblem(Number(id)),
    openOutletMap: () => toast("地图为演示降级实现：显示网点列表"),
    guide: () => go("u-guide"),
    adminNav: () => go(id),
    adminLogout: () => { localStorage.removeItem("locker_admin"); state.adminToken = null; go("m-login"); },
    kioskStore: () => go("k-store"),
    kioskPickup: () => go("k-pickup"),
    kioskHome: () => go("k-standby"),
    kioskTip: () => toast("请在用户端（手机端）完成选柜与支付下单"),
  };
  if (acts[act]) { e.preventDefault(); acts[act](); }
});

function requireLogin() {
  if (hasToken()) return true;
  toast("请先登录");
  go("u-login");
  return false;
}

function openCityPicker() {
  api("/api/cities").then((cities) => {
    modal('<div class="bold14 mb10">选择城市</div>' + cities.map((c) =>
      '<button class="row btn city-opt" data-action="pickCity" data-id="' + esc(c.city) + '" style="justify-content:space-between;width:100%;margin-bottom:8px"><span>' + esc(c.city) + '</span><span class="faint tiny">空闲 ' + c.idle + ' 格</span></button>').join(""));
    rootPickCity();
  }).catch((err) => toast(err.message));
}
function rootPickCity() {
  document.querySelectorAll("[data-action='pickCity']").forEach((b) => {
    b.onclick = () => { state.city = b.dataset.id; closeModal(); render(); };
  });
}

function openOutletDetail(id) {
  api("/api/outlets/" + id).then((o) => { state.outlet = o; go("u-detail"); }).catch((err) => toast(err.message));
}

function chooseCoupon(cid) {
  state.couponId = cid;
  refreshQuote().then(() => render());
}

/* 计费试算 */
async function refreshQuote() {
  const rule = await api("/api/outlets/" + state.outlet.id + "/rule");
  state.rule = rule;
  state.quote = await api("/api/quote", "POST", { outletId: state.outlet.id, size: state.spec, payMode: state.payMode });
}

/* 上报柜格异常 */
function reportProblem(orderId) {
  const problems = ["柜门未弹开", "柜门无法关闭", "柜格内有异物", "柜格灯不亮", "其他问题"];
  modal('<div class="bold14 mb10">选择问题类型</div>' + problems.map((p) =>
    '<button class="row btn prob-opt" data-p="' + esc(p) + '" style="width:100%;margin-bottom:8px;justify-content:flex-start">' + esc(p) + '</button>').join("") +
    '<div class="faint tiny mt10">提交后柜格将标记异常，订单关闭并全额退款</div>');
  document.querySelectorAll(".prob-opt").forEach((b) => {
    b.onclick = () => {
      const p = b.dataset.p;
      closeModal();
      api("/api/orders/" + orderId + "/report", "POST", { problem: p }).then((r) => {
        toast("已上报，退款 ¥" + yuan2(r.refund));
        render();
      }).catch((err) => toast(err.message));
    };
  });
}

/* ============ 图标/小组件 ============ */
const sizeName = { S: "小格", M: "中格", L: "大格" };
const statusName = { idle: "空闲", occupied: "占用", fault: "异常" };
function statusChip(st) {
  const map = { idle: "chip-ok", occupied: "chip-warn", fault: "chip-err" };
  return '<span class="chip ' + (map[st] || "") + '">' + statusName[st] + "</span>";
}

/* ================================================================
 *  用户端
 * ================================================================ */
function tabbar(active) {
  // 用户端不提供取件入口：取件统一在柜机触屏端凭取件码完成
  return '<div class="tabbar">' +
    '<button class="' + (active === "home" ? "on" : "") + '" data-action="nav" data-id="u-home">' + I.box + "<span>首页</span></button>" +
    '<button class="' + (active === "orders" ? "on" : "") + '" data-action="nav" data-id="u-orders">' + I.list + "<span>订单</span></button>" +
    '<button class="' + (active === "mine" ? "on" : "") + '" data-action="nav" data-id="u-mine">' + I.user + "<span>我的</span></button>" +
    "</div>";
}
function appHeader(title, backTo) {
  return '<div class="app-header">' + (backTo ? '<button class="icon-btn" data-action="nav" data-id="' + backTo + '">' + I.back + "</button>" : "") + '<div class="ttl">' + esc(title) + "</div></div>";
}

routes["u-login"] = function () {
  const mode = state.loginMode || "password";
  return appHeader("登录", "u-home") +
    '<div class="app-pad"><div class="h2">手机号登录 / 注册</div>' +
    '<div class="login-tabs mt12"><button class="login-tab ' + (mode === "password" ? "on" : "") + '" data-m="password">密码登录</button>' +
    '<button class="login-tab ' + (mode === "code" ? "on" : "") + '" data-m="code">验证码登录</button></div>' +
    '<div class="card mt12"><label class="lbl">手机号</label><input class="input" id="phone" maxlength="11" placeholder="请输入11位手机号" inputmode="numeric">' +
    (mode === "password"
      ? '<label class="lbl mt12">登录密码</label><input class="input" id="pwd" type="password" placeholder="请输入登录密码">' +
        '<div class="tiny mt5"><span class="link" id="forgotPwd">忘记密码？用验证码登录 ›</span></div>'
      : '<div class="row gap8 mt12"><input class="input flex1" id="code" maxlength="6" placeholder="验证码" inputmode="numeric"><button class="btn" id="sendCode">发送验证码</button></div>' +
        '<div class="faint tiny mt5">演示环境验证码固定为 1234；首次登录请用验证码，登录后可在「我的」设置密码</div>') +
    '<label class="row gap8 mt20" style="font-size:12px;color:var(--muted);align-items:flex-start;cursor:pointer"><input type="checkbox" id="agreeChk" style="margin-top:2px"><span>我已阅读并同意 <span class="link" data-action="nav" data-id="u-guide">《用户协议》</span> 与《隐私政策》</span></label>' +
    '<button class="btn btn-primary mt20" id="doLogin">' + (mode === "password" ? "登 录" : "登录 / 注册") + '</button></div></div>';
};

document.addEventListener("click", (e) => {
  const tab = e.target.closest(".login-tab");
  if (tab) { state.loginMode = tab.dataset.m; render(); return; }
  if (e.target.closest("#forgotPwd")) { state.loginMode = "code"; render(); return; }
  if (e.target.closest("#sendCode")) {
    const phone = $("#phone").value.trim();
    api("/api/user/send-code", "POST", { phone }).then(() => toast("验证码已发送（演示：1234）")).catch((err) => toast(err.message));
  }
  if (e.target.closest("#doLogin")) {
    const phone = $("#phone").value.trim();
    const mode = state.loginMode || "password";
    const code = mode === "code" ? ($("#code") ? $("#code").value.trim() : "") : "";
    const password = mode === "password" ? ($("#pwd") ? $("#pwd").value : "") : "";
    const agree = $("#agreeChk").checked;
    const body = { phone, mode, agree };
    if (mode === "code") body.code = code;
    else body.password = password;
    api("/api/user/login", "POST", body).then((r) => {
      saveToken(r.token);
      if (r.avatar) { state.avatar = r.avatar; localStorage.setItem("locker_avatar", r.avatar); }
      toast("登录成功");
      if (!r.hasLoginPwd) {
        // 首次用验证码登录：引导设置密码，下次即可密码登录
        setTimeout(() => {
          modal('<div class="bold14 mb10">设置登录密码</div><div class="faint tiny mb10">下次可用「手机号+密码」快速登录，忘记密码仍可用验证码</div>' +
            '<input class="input" id="setLoginPwd" type="password" placeholder="至少6位登录密码">' +
            '<button class="btn btn-primary mt12" id="setLoginPwdGo" style="width:100%">保存密码</button>' +
            '<button class="btn mt8" id="skipPwd" style="width:100%">暂不设置</button>');
          $("#setLoginPwdGo").onclick = async () => {
            const p = $("#setLoginPwd").value;
            if (p.length < 6) { toast("密码至少6位"); return; }
            try { await api("/api/user/login-pwd", "POST", { password: p }); closeModal(); toast("密码已设置，下次可直接登录"); }
            catch (err) { toast(err.message); }
          };
          $("#skipPwd").onclick = () => closeModal();
        }, 600);
      } else go("u-home");
    }).catch((err) => {
      toast(err.message, 3500);
      const box = $("#uLoginErr");
      if (box) { box.textContent = "✕ " + err.message; box.style.display = "block"; }
      else { const n = document.createElement("div"); n.id = "uLoginErr"; n.className = "login-err"; n.textContent = "✕ " + err.message; $("#doLogin").before(n); }
    });
  }
});

routes["u-home"] = async function () {
  const cities = await api("/api/cities");
  const outlets = await api("/api/outlets?city=" + encodeURIComponent(state.city));
  const cur = cities.find((c) => c.city === state.city) || cities[0];
  state._after = () => {
    const box = $("#cabMini");
    if (!box) return;
    const first = outlets[0];
    if (!first) { box.innerHTML = '<div class="faint tiny">暂无网点</div>'; return; }
    api("/api/outlets/" + first.id).then((d) => {
      box.innerHTML = cabinetGrid(d.cabinets || [], false) + cabinetLegend(d.byStatus);
      const card = $("#cabStatusCard");
      if (card) card.onclick = (e) => {
        if (e.target.closest(".link")) return;
        modal('<div class="bold14 mb10">柜格使用情况 · ' + esc(first.name) + '</div>' +
          cabinetGrid(d.cabinets || [], true) + cabinetLegend(d.byStatus) +
          '<div class="faint tiny mt12" style="line-height:1.7">空闲柜格可直接寄存；异常柜格暂不开放。用户端取件请在柜机端凭取件码操作。</div>' +
          '<button class="btn btn-primary mt12" id="cabOk" style="width:100%">我知道了</button>');
        $("#cabOk").onclick = closeModal;
      };
    }).catch(() => { box.innerHTML = '<div class="faint tiny">柜格状态加载失败</div>'; });
  };
  return appHeader("优小宝寄存") +
    '<div class="app-pad">' +
    '<div class="greeting"><div class="hi">欢迎使用智能寄存</div><div class="location" data-action="city" style="cursor:pointer">' + I.pin + " " + esc(cur.city) + ' <span class="faint tiny">切换 ›</span></div></div>' +
    '<div class="hero-card"><div class="lbl">' + esc(cur.city) + ' · 空闲柜格</div><div class="big mono">' + cur.idle + ' <span style="font-size:14px;font-weight:600">格</span></div><div style="font-size:11.5px;opacity:.85">共 ' + outlets.length + ' 个网点可用</div></div>' +
    '<div class="hero-actions"><button class="hero-act" data-action="spec"><div class="ic">' + I.box + '</div><b>立即存件</b><span>选柜型 · 在线支付</span></button>' +
    '<button class="hero-act" data-action="nav" data-id="u-orders"><div class="ic">' + I.list + '</div><b>我的订单</b><span>查看取件码 / 补交</span></button></div>' +
    '<div class="faint tiny mt8" style="text-align:center">取件请到柜机触屏端，输入订单中的 6 位取件码</div>' +
    '<div class="card mt12" id="cabStatusCard"><div class="row" style="justify-content:space-between"><b>柜机状态</b><span class="link" data-action="nav" data-id="u-outlets">查看详情 ›</span></div>' +
    '<div id="cabMini" class="mt8"><div class="faint tiny">加载中…</div></div></div>' +
    '<div class="section"><div class="section-h"><b>附近网点</b><span class="link" data-action="nav" data-id="u-outlets">全部 ›</span></div>' +
    outlets.slice(0, 3).map((o) => outletRow(o)).join("") + "</div>" +
    '<div class="card mt12" data-action="nav" data-id="u-chat" style="display:flex;align-items:center;gap:11px;cursor:pointer"><div style="width:34px;height:34px;border-radius:10px;background:var(--bg-2);display:grid;place-items:center">' + I.chat + '</div>' +
    '<div style="flex:1"><b style="font-size:12.5px">在线客服</b><div class="faint tiny">智能客服 7×24 · 人工 09:00-21:00</div></div><span class="link">进入 ›</span></div>' +
    "</div>" + tabbar("home");
};

function outletRow(o) {
  return '<div class="card outlet-row" data-action="outlet" data-id="' + o.id + '" style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px;cursor:pointer">' +
    '<div style="min-width:0"><b style="font-size:13px">' + esc(o.name) + '</b><div class="faint tiny" style="margin-top:3px">' + esc(o.address || o.city) + "</div></div>" +
    '<div style="text-align:right"><div class="mono" style="font-weight:700">空闲 ' + o.idle + '</div><div class="tiny faint">¥' + yuan(o.priceS) + "/时起</div></div></div>";
}

let outletMap = null;
function initOutletMap(outlets) {
  const el = $("#olMap");
  if (!el) return;
  if (typeof L === "undefined" || !outlets.length) {
    el.innerHTML = '<div class="map-fallback"><b>附近网点分布图</b><div class="map-fallback-pins">' +
      outlets.slice(0, 6).map((o, i) => '<span class="pin" style="left:' + (18 + (i % 3) * 27) + '%;top:' + (22 + Math.floor(i / 3) * 42) + '%">' + (o.idle || 0) + '格</span>').join("") + '</div><div class="faint tiny">地图服务不可用，显示示意图</div></div>';
    return;
  }
  if (outletMap) { outletMap.remove(); outletMap = null; }
  const pts = outlets.filter((o) => o.lat && o.lng);
  if (!pts.length) return;
  outletMap = L.map(el, { zoomControl: false }).setView([pts[0].lat, pts[0].lng], 13);
  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    attribution: "&copy; OpenStreetMap contributors", maxZoom: 18,
  }).addTo(outletMap);
  pts.forEach((o) => {
    L.marker([o.lat, o.lng]).addTo(outletMap)
      .bindPopup('<b>' + esc(o.name) + '</b><br>空闲 ' + (o.idle || 0) + ' 格<br>' + esc(o.address || ""))
      .on("click", () => openOutletDetail(o.id));
  });
}
routes["u-outlets"] = async function () {
  const outlets = await api("/api/outlets?city=" + encodeURIComponent(state.city)).catch(() => []);
  state._after = () => {
    initOutletMap(outlets);
    const inp = $("#olSearch");
    if (inp) inp.oninput = (e) => {
      const kw = e.target.value.trim();
      api("/api/outlets/search?q=" + encodeURIComponent(kw)).then((list) => {
        const el = $("#olList");
        if (el) el.innerHTML = list.map(outletRow).join("");
      });
    };
  };
  return appHeader("附近寄存 · " + esc(state.city), "u-home") +
    '<div class="app-pad"><input class="input" id="olSearch" placeholder="搜索网点名称 / 地址">' +
    '<div id="olMap" class="ol-map mt12"></div>' +
    '<div id="olList" class="mt12">' + outlets.map(outletRow).join("") + "</div></div>" + tabbar("");
};

routes["u-detail"] = function () {
  const o = state.outlet;
  if (!o) { go("u-home"); return ""; }
  const s = o.idleBySize || {};
  return appHeader(o.name, "u-outlets") +
    '<div class="app-pad"><div class="hero-card"><div class="lbl">' + esc(o.city) + " · " + esc(o.address) + '</div><div class="big mono">' + (s.S || 0) + "/" + (s.M || 0) + "/" + (s.L || 0) + ' <span style="font-size:13px">小/中/大 空闲</span></div></div>' +
    '<div class="card mt12"><div class="bold14 mb10">柜格状态</div>' +
    '<div class="row gap12" style="font-size:12px"><span>' + statusChip("idle") + " 空闲 " + (o.byStatus ? o.byStatus.idle : 0) + "</span><span>" + statusChip("occupied") + " 占用 " + (o.byStatus ? o.byStatus.occupied : 0) + "</span><span>" + statusChip("fault") + " 异常 " + (o.byStatus ? o.byStatus.fault : 0) + "</span></div></div>" +
    '<div class="card mt12"><div class="bold14 mb10">收费规则</div><div class="pay-row"><span class="muted">小格</span><b>¥' + yuan2(o.priceS) + '/时 · 押金 ¥' + yuan2(1000) + '</b></div>' +
    '<div class="pay-row"><span class="muted">中格</span><b>¥' + yuan2(o.priceM) + '/时 · 押金 ¥' + yuan2(2000) + '</b></div>' +
    '<div class="pay-row" style="border:0"><span class="muted">大格</span><b>¥' + yuan2(o.priceL) + '/时 · 押金 ¥' + yuan2(3000) + "</b></div></div>" +
    '<button class="btn btn-primary mt16" data-action="spec">立即存件</button></div>' + tabbar("");
};

routes["u-spec"] = async function () {
  if (!requireLogin()) return "";
  const o = state.outlet || (await api("/api/outlets?city=" + encodeURIComponent(state.city)).then((l) => l[0]));
  if (!o) { toast("当前城市暂无可用网点"); return ""; }
  state.outlet = o;
  const detail = await api("/api/outlets/" + o.id);
  // 价格来自收费规则接口（detail 不含价格字段，误用会导致显示 ¥0.00）
  const rule = await api("/api/outlets/" + o.id + "/rule").catch(() => null);
  state.outlet = Object.assign({}, detail, rule || {});
  const s = detail.idleBySize || {};
  state._after = () => {
    document.querySelectorAll(".spec-card").forEach((el) => {
      if (el.dataset.empty === "1") return; // 0 格不可选
      el.onclick = () => { state.spec = el.dataset.size; render(); };
    });
    document.querySelectorAll("input[name=pm]").forEach((el) => { el.onchange = () => { state.payMode = el.value; }; });
    $("#toPay").onclick = async () => {
      if (!state.spec) { toast("请选择柜型"); return; }
      // 二次校验库存，避免页面停留期间库存变化导致"0格仍可下单"
      const fresh = await api("/api/outlets/" + o.id).catch(() => null);
      if (fresh && fresh.idleBySize && (fresh.idleBySize[state.spec] || 0) <= 0) { toast("该柜型已无空闲柜格，请选择其他规格"); render(); return; }
      await refreshQuote();
      go("u-pay");
    };
  };
  return appHeader("选择柜型", "u-home") + '<div class="app-pad">' +
    '<div class="h2">选择柜型与计费方式</div>' +
    ["S", "M", "L"].map((k) => {
      const empty = (s[k] || 0) <= 0;
      return '<button class="card spec-card ' + (state.spec === k ? "on" : "") + (empty ? " disabled" : "") + '" data-size="' + k + '" data-empty="' + (empty ? "1" : "0") + '" style="width:100%;text-align:left;margin-bottom:10px;display:flex;justify-content:space-between;align-items:center' + (empty ? ';opacity:.5' : '') + '">' +
      '<div><b>' + sizeName[k] + '</b><div class="faint tiny mt5">' + (empty ? '已无空闲柜格' : '空闲 ' + s[k] + ' 格') + '</div></div>' +
      '<div class="mono" style="font-weight:700">¥' + yuan2(state.outlet["price" + k] || o["price" + k] || 0) + "/时</div></button>";
    }).join("") +
    '<div class="card mt12"><div class="bold14 mb10">计费方式</div>' +
    '<label class="row gap8" style="margin-bottom:8px;cursor:pointer"><input type="radio" name="pm" value="prepaid" ' + (state.payMode === "prepaid" ? "checked" : "") + '><span>预付模式（1小时费用+押金，结束订单多退少补）</span></label>' +
    '<label class="row gap8" style="cursor:pointer"><input type="radio" name="pm" value="deposit" ' + (state.payMode === "deposit" ? "checked" : "") + '><span>押金模式（先付押金，按实际时长扣费）</span></label></div>' +
    '<button class="btn btn-primary mt16" id="toPay">去支付</button></div>' + tabbar("");
};

routes["u-pay"] = async function () {
  const q = state.quote || {};
  let coupons = [];
  try { coupons = (await api("/api/user/coupons")).usable || []; } catch (e) { coupons = []; }
  const sel = coupons.find((c) => c.id === state.couponId);
  state._after = () => {
    $("#doPay").onclick = async () => {
      try {
        const r = await api("/api/orders", "POST", { outletId: state.outlet.id, size: state.spec, payMode: state.payMode, couponInstanceId: state.couponId });
        state.lastOrder = r;
        state.openStep = 0;
        go("u-open");
      } catch (err) { toast(err.message); }
    };
  };
  return appHeader("确认订单", "u-spec") +
    '<div class="app-pad"><div class="h2">确认支付</div><div class="card mt16">' +
    '<div class="pay-row"><span class="muted">寄存柜</span><b>' + esc(state.outlet ? state.outlet.name : "") + "</b></div>" +
    '<div class="pay-row"><span class="muted">柜型</span><b>' + (sizeName[state.spec] || state.spec || "") + "</b></div>" +
    '<div class="pay-row"><span class="muted">计费方式</span><b>' + (q.modeName || "") + "</b></div>" +
    '<div class="pay-row"><span class="muted">单价 / 押金</span><b>¥' + yuan2(q.price) + " / 押金 ¥" + yuan2(q.deposit) + "</b></div>" +
    (q.cap ? '<div class="pay-row"><span class="muted">24 小时封顶</span><b>¥' + yuan2(q.cap) + "</b></div>" : "") +
    '<div class="pay-row"><span class="muted">免费时长</span><b>' + (q.freeMinutes || 0) + " 分钟</b></div>" +
    '<button class="pay-row" data-action="couponSelect" style="width:100%;text-align:left"><span class="muted">优惠卡券</span><b class="link">' + (sel ? esc(sel.name) + "（结束订单时结算）" : "选择优惠券 ›") + "</b></button>" +
    '<div class="pay-row" style="border:0"><span class="muted">' + esc(q.payName || "应付") + '</span><b class="price">¥' + yuan2(q.payNow) + "</b></div></div>" +
    '<button class="btn btn-primary mt20" id="doPay">模拟支付 ¥' + yuan2(q.payNow) + "</button>" +
    '<div class="faint tiny mt10" style="text-align:center">支付渠道为模拟实现，不产生真实扣款</div></div>';
};

routes["u-coupon-select"] = async function () {
  let coupons = [];
  try { coupons = (await api("/api/user/coupons")).usable || []; } catch (e) {}
  state._after = () => {
    $("#noCoupon").onclick = () => { state.couponId = 0; refreshQuote().then(() => go("u-pay")); };
    document.querySelectorAll(".cp-card").forEach((el) => {
      el.onclick = () => { state.couponId = Number(el.dataset.cid); refreshQuote().then(() => go("u-pay")); };
    });
  };
  return appHeader("选择优惠券", "u-pay") +
    '<div class="app-pad">' +
    '<button class="card" id="noCoupon" style="width:100%;text-align:left;margin-bottom:10px">不使用优惠券</button>' +
    (coupons.length ? coupons.map((c) =>
      '<button class="card cp-card" data-cid="' + c.id + '" style="width:100%;text-align:left;margin-bottom:10px"><b>' + esc(c.name) + '</b><span class="chip chip-ok ml8">' + esc(c.type) + "</span>" +
      '<div class="faint tiny mt5">' + esc(c.valueDesc) + " · " + esc(c.validFrom) + " ~ " + esc(c.validTo) + (c.scope === "outlet" ? " · 仅限指定网点" : " · 全网点通用") + "</div></button>").join("")
      : '<div class="card mt12 faint">暂无可用优惠券</div>') + "</div>";
};

/* 存件流程状态机：支付成功 → 开柜动画 → 放入物品 → 关柜动画 → 完成 */
function openStepView(step, o) {
  const steps = ["支付成功", "开柜门", "放入物品", "关柜门", "存件完成"];
  const bar = '<div class="step-bar">' + steps.map((s, i) =>
    '<div class="step ' + (i < step ? "done" : i === step ? "on" : "") + '">' + (i < step ? "✓" : i + 1) + "</div>").join("") + '</div><div class="step-lbl">' + steps[step] + '</div>';
  const door = (st) => '<div class="locker-scene"><div class="locker-body"><div class="locker-door ' + st + '">' +
    '<div class="locker-handle"></div></div><div class="locker-slot"></div></div></div>';
  const info = '<div class="card mt12"><div class="pay-row"><span class="muted">柜格</span><b class="mono">' + esc(o.lockerCode) + '</b></div>' +
    '<div class="pay-row"><span class="muted">取件码</span><b class="mono" style="font-size:18px;letter-spacing:3px">' + esc(o.code) + '</b></div>' +
    '<div class="pay-row" style="border:0"><span class="muted">支付金额</span><b class="price">¥' + yuan2(o.paid) + '</b></div></div>';
  switch (step) {
    case 0:
      return '<div class="app-pad" style="text-align:center"><div class="big-ok">✓ 支付成功</div><div class="faint tiny mt5">已为您分配柜格，准备开柜</div>' + bar + info +
        '<button class="btn btn-primary mt16" id="openGo" style="width:100%">开 柜</button></div>';
    case 1:
      return '<div class="app-pad" style="text-align:center"><div class="h2">正在打开柜门…</div>' + bar + door("opening") + info +
        '<div class="faint tiny mt12">如柜门未弹开，可上报异常，系统自动切换备用柜格</div>' +
        '<button class="btn mt12" id="openFail" style="width:100%">柜门未开启？上报并换柜</button></div>';
    case 2:
      return '<div class="app-pad" style="text-align:center"><div class="big-ok">柜门已弹开</div>' + bar + door("open") + info +
        '<div class="faint tiny mt12">请确保柜门完全打开，放入物品后关闭柜门</div>' +
        '<button class="btn btn-primary mt16" id="closeGo" style="width:100%">物品已放入，关闭柜门</button>' +
        '<button class="btn mt8" id="openFail2" style="width:100%">柜门异常？上报并换柜</button></div>';
    case 3:
      return '<div class="app-pad" style="text-align:center"><div class="h2">正在关闭柜门…</div>' + bar + door("closing") + info +
        '<button class="btn mt12" id="closeFail" style="width:100%">柜门关不上？上报并换柜</button></div>';
    default:
      return '<div class="app-pad" style="text-align:center"><div class="big-ok">✓ 柜门已关闭，存件完成</div>' + bar + door("closed") + info +
        '<div class="card mt12 faint tiny" style="line-height:1.9">取件码请妥善保管，也可在「订单」中查看。柜机端凭取件码取件。</div>' +
        '<button class="btn btn-primary mt16" data-action="nav" data-id="u-orders" style="width:100%">查看订单</button></div>';
  }
}

routes["u-open"] = function () {
  const o = state.lastOrder;
  if (!o) { go("u-home"); return ""; }
  state.openStep = state.openStep || 0;
  const makeDetail = () => appHeader("存件指引", "u-home") + openStepView(state.openStep, o);
  const goStep = (n) => {
    state.openStep = n;
    const box = $("#app");
    if (box) { box.innerHTML = makeDetail(); bindStep(); }
  };
  const reportSwitch = () => {
    state.switchCount = (state.switchCount || 0) + 1;
    const m = String(o.lockerCode || "A-001").match(/^(.*?)(\d+)$/);
    o.lockerCode = m ? m[1] + (Number(m[2]) + 1) : String(o.lockerCode) + "1";
    toast("已上报柜门异常，自动切换备用柜格：" + o.lockerCode);
    goStep(0);
  };
  const bindStep = () => {
    const b1 = $("#openGo"), b2 = $("#closeGo");
    if (b1) b1.onclick = () => { goStep(1); setTimeout(() => goStep(2), 1200); };
    if (b2) b2.onclick = () => { goStep(3); setTimeout(() => goStep(4), 1200); };
    ["#openFail", "#openFail2", "#closeFail"].forEach((sel) => { const b = $(sel); if (b) b.onclick = reportSwitch; });
  };
  state._after = bindStep;
  return makeDetail();
};

routes["u-orders"] = async function () {
  if (!requireLogin()) return "";
  let cur = [], hist = [];
  try { cur = await api("/api/orders/current"); hist = await api("/api/orders/history"); } catch (e) {}
  return appHeader("我的订单", "u-home") + '<div class="app-pad">' +
    (cur.length ? '<div class="section-h"><b>进行中</b></div>' + cur.map(orderCard).join("") : "") +
    (hist.length ? '<div class="section-h mt16"><b>历史订单</b></div>' + hist.map(orderCard).join("") : "") +
    (!cur.length && !hist.length ? '<div class="card mt16 faint">暂无订单，去首页寄存吧</div>' : "") +
    "</div>" + tabbar("orders");
};

function orderCard(o) {
  const active = o.status === "active";
  return '<div class="card order-card" style="margin-bottom:10px">' +
    '<div class="row" style="justify-content:space-between"><b>订单 ' + esc(o.orderNo) + "</b>" +
    '<span class="chip ' + (active ? "chip-ok" : "") + '">' + (active ? "进行中" : "已结束") + "</span></div>" +
    '<div class="faint tiny mt5">柜型 ' + sizeName[o.size] + " · " + (o.payMode === "deposit" ? "押金模式" : "预付模式") + (o.lockerCode ? " · 柜格 " + esc(o.lockerCode) : "") + "</div>" +
    (o.code ? '<div class="pickup-code-box mt10"><span class="tiny faint">取件码</span><b class="mono">' + esc(o.code) + '</b><button class="btn sm copy-code" data-code="' + esc(o.code) + '">复制</button></div>' : "") +
    (active
      ? '<div class="faint tiny mt8">' + (Number(o.extra) > 0 ? '已结算 · 需补交 ¥' + yuan2(o.extra) + '（请在柜机端取件后于此补交）' : '寄存中 · 请到柜机端凭取件码取件') + '</div>' +
        '<div class="row gap8 mt10">' +
        (Number(o.extra) > 0 ? '<button class="btn btn-primary flex1 op-extra" data-oid="' + o.id + '">补交 ¥' + yuan2(o.extra) + '</button>' : '') +
        '<button class="btn flex1 op-report" data-oid="' + o.id + '">上报柜格异常</button></div>'
      : '<div class="faint tiny mt8">费用 ¥' + yuan2(o.fee) + (o.refund ? " · 退款 ¥" + yuan2(o.refund) : "") + (o.extra ? " · 补交 ¥" + yuan2(o.extra) : "") + (o.couponDiscount ? " · 券抵 ¥" + yuan2(o.couponDiscount) : "") +
        (o.settleNote ? "<br>" + esc(o.settleNote) : "") + "</div>") +
    "</div>";
}

document.addEventListener("click", (e) => {
  const cp = e.target.closest(".copy-code");
  if (cp) {
    const txt = cp.dataset.code || "";
    copyText(txt).then((ok) => {
      toast(ok ? "✓ 复制成功！取件码 " + txt + " 已复制到剪贴板" : "取件码：" + txt + "（请手动复制）", 3000);
      if (ok) {
        const old = cp.textContent;
        cp.textContent = "✓ 已复制";
        cp.style.background = "#2E7D32";
        cp.style.color = "#fff";
        setTimeout(() => { if (cp.isConnected) { cp.textContent = old; cp.style.background = ""; cp.style.color = ""; } }, 1800);
      }
    });
    return;
  }
  const ex = e.target.closest(".op-extra");
  if (ex) {
    const id = Number(ex.dataset.oid);
    api("/api/orders/" + id + "/pay-extra", "POST", {}).then(() => { toast("补交完成，订单已结束"); render(); }).catch((err) => toast(err.message));
    return;
  }
  const rp = e.target.closest(".op-report");
  if (rp) { reportProblem(Number(rp.dataset.oid)); return; }
  const dn = e.target.closest(".op-done");
  if (dn) {
    const id = Number(dn.dataset.oid);
    api("/api/orders/" + id + "/pickup-done", "POST", {}).then((r) => {
      if (Number(r.extra) > 0) {
        modal('<div class="bold14">需补交差额</div><div class="mt10">实际费用超出预付金额，需补交 <b class="price">¥' + yuan2(r.extra) + "</b></div><div class='faint tiny mt8'>" + esc(r.note) + "</div>" +
          '<button class="btn btn-primary mt16" id="payExtra" style="width:100%">模拟补交支付</button>');
        $("#payExtra").onclick = () => {
          api("/api/orders/" + id + "/pay-extra", "POST", {}).then(() => { closeModal(); toast("补交完成，订单结束"); render(); }).catch((err) => toast(err.message));
        };
      } else {
        toast("结算完成：" + (r.kind === "free" ? "免费时长内，全额退款 ¥" + yuan2(r.refund) : r.kind === "refund" ? "退款 ¥" + yuan2(r.refund) : "无需补退"));
        render();
      }
    }).catch((err) => toast(err.message));
  }
});

// 用户端不提供取件流程：取件统一在柜机触屏端凭 6 位取件码完成。
// 用户端仅保留：查看取件码（订单卡片）、补交差额、上报异常。

routes["u-mine"] = function () {
  state._after = () => {
    const av = $("#changeAvatar");
    if (av) av.onclick = () => {
      const saveAvatar = async (a) => {
        state.avatar = a;
        localStorage.setItem("locker_avatar", a);
        const box = $("#myAvatar");
        if (box) box.innerHTML = avatarHTML(a, "user-avatar");
        closeModal();
        try { await api("/api/user/avatar", "POST", { avatar: a }); toast("✓ 头像已更新"); }
        catch (err) { toast("头像已在本地保存"); }
      };
      modal('<div class="bold14 mb10">更换头像</div>' +
        '<button class="btn btn-primary mt8" id="uploadAvatarBtn" style="width:100%">📷 上传本地图片作为头像</button>' +
        '<input type="file" id="avatarFile" accept="image/*" style="display:none">' +
        '<div class="faint tiny mt10">或选择一个表情头像：</div>' +
        '<div class="avatar-picker">' + AVATARS.map((a) => '<button class="avatar-opt ' + (a === state.avatar ? "on" : "") + '" data-a="' + a + '">' + a + '</button>').join("") + '</div>');
      document.querySelectorAll(".avatar-opt").forEach((b) => { b.onclick = () => saveAvatar(b.dataset.a); });
      $("#uploadAvatarBtn").onclick = () => $("#avatarFile").click();
      $("#avatarFile").onchange = async (e) => {
        const file = e.target.files && e.target.files[0];
        if (!file) return;
        if (!file.type.startsWith("image/")) { toast("请选择图片文件"); return; }
        if (file.size > 5 * 1024 * 1024) { toast("图片请小于 5MB"); return; }
        toast("正在处理图片…");
        try {
          const dataURL = await imageToAvatar(file);
          await saveAvatar(dataURL);
        } catch (err) { toast(err.message || "图片处理失败"); }
      };
    };
    const lpBtn = $("#setLoginPwdEntry");
    if (lpBtn) lpBtn.onclick = () => {
      modal('<div class="bold14 mb10">设置登录密码</div><div class="faint tiny mb10">下次可用「手机号+密码」登录；忘记密码仍可用验证码</div>' +
        '<input class="input" id="setLoginPwd" type="password" placeholder="至少6位登录密码">' +
        '<button class="btn btn-primary mt12" id="setLoginPwdGo" style="width:100%">保存</button>');
      $("#setLoginPwdGo").onclick = async () => {
        const p = $("#setLoginPwd").value;
        if (p.length < 6) { toast("密码至少6位"); return; }
        try { await api("/api/user/login-pwd", "POST", { password: p }); closeModal(); toast("登录密码已设置"); }
        catch (err) { toast(err.message); }
      };
    };
    const btn = $("#setPwd");
    if (btn) btn.onclick = () => {
      modal('<div class="bold14 mb10">设置取件密码</div><input class="input" id="mNewPwd" type="password" placeholder="至少6位"><button class="btn btn-primary mt12" id="mSetPwd" style="width:100%">保存</button>');
      $("#mSetPwd").onclick = () => {
        const pwd = $("#mNewPwd").value.trim();
        if (pwd.length < 6) { toast("取件密码至少6位"); return; }
        api("/api/user/pickup-pwd", "POST", { password: pwd }).then(() => { closeModal(); toast("取件密码已保存"); }).catch((err) => toast(err.message));
      };
    };
  };
  return appHeader("我的", "u-home") +
    '<div class="app-pad"><div class="profile-card"><div class="profile-top"><span id="myAvatar">' + avatarHTML(state.avatar, "user-avatar") + '</span>' +
    '<div class="profile-info"><div class="profile-name">已登录用户</div><div class="profile-sub">优惠券 · 取件码 · 在线客服</div></div>' +
    '<button class="profile-edit" id="changeAvatar">更换头像</button></div></div>' +
    '<div class="card mt12"><div class="bold14">账户</div>' +
    '<button class="pay-row" data-action="nav" data-id="u-coupons" style="width:100%;text-align:left"><span class="muted">优惠券</span><b class="link">查看 ›</b></button>' +
    '<button class="pay-row" id="setPwd" style="width:100%;text-align:left"><span class="muted">取件密码</span><b class="link">设置 ›</b></button>' +
    '<button class="pay-row" id="setLoginPwdEntry" style="width:100%;text-align:left"><span class="muted">登录密码</span><b class="link">设置 ›</b></button>' +
    '<button class="pay-row" data-action="nav" data-id="u-chat" style="width:100%;text-align:left;border:0"><span class="muted">在线客服</span><b class="link">咨询 ›</b></button></div>' +
    '<button class="btn mt16" id="doLogout" style="width:100%">退出登录</button></div>' + tabbar("mine");
};
document.addEventListener("click", (e) => {
  if (e.target.closest("#doLogout")) {
    api("/api/user/logout", "POST", {}).catch(() => {});
    saveToken("");
    toast("已退出");
    go("u-login");
  }
});

routes["u-coupons"] = async function () {
  if (!requireLogin()) return "";
  const [data, avail] = await Promise.all([
    api("/api/user/coupons").catch(() => ({ usable: [], expired: [] })),
    api("/api/user/coupons/available").catch(() => ({ coupons: [] })),
  ]);
  const card = (c, dead) =>
    '<div class="card cp-card' + (dead ? " dead" : "") + '" style="margin-bottom:10px;opacity:' + (dead ? ".55" : "1") + '"><b>' + esc(c.name) + '</b><span class="chip chip-ok ml8">' + esc(c.type) + "</span>" +
    '<div class="mt5 mono" style="font-weight:700">' + esc(c.valueDesc) + "</div>" +
    '<div class="faint tiny mt5">' + (dead ? "已过期/停用 · " : "") + esc(c.validFrom) + " ~ " + esc(c.validTo) + " · " + (c.scope === "outlet" ? "限指定网点" : "全网点通用") + "</div></div>";
  const claimCard = (c) =>
    '<div class="card cp-claim" style="margin-bottom:10px"><div class="row" style="justify-content:space-between"><div><b>' + esc(c.name) + '</b> <span class="chip chip-ok ml8">' + esc(c.valueDesc) + '</span>' +
    '<div class="faint tiny mt5">有效期至 ' + esc(c.validTo) + ' · 剩余 ' + c.left + '/' + c.total + ' 张</div></div>' +
    '<button class="btn btn-sm claim-btn" data-id="' + c.id + '" ' + (c.soldOut || c.claimed ? "disabled" : "") + '>' +
    (c.claimed ? "已领取" : c.soldOut ? "已抢完" : "领取") + '</button></div></div>';
  state._after = () => {
    document.querySelectorAll(".claim-btn").forEach((b) => {
      b.onclick = () => {
        api("/api/user/coupons/" + b.dataset.id + "/claim", "POST", {}).then((r) => {
          toast("✓ " + (r.note || "领取成功")); render();
        }).catch((err) => toast(err.message, 3500));
      };
    });
  };
  return appHeader("优惠券", "u-mine") + '<div class="app-pad">' +
    '<div class="section-h"><b>领券中心</b></div>' +
    (avail.coupons.map(claimCard).join("") || '<div class="faint tiny">暂无在售优惠券</div>') +
    '<div class="section-h mt16"><b>可使用</b></div>' + (data.usable.map((c) => card(c, false)).join("") || '<div class="faint tiny">暂无</div>') +
    '<div class="section-h mt16"><b>已过期</b></div>' + (data.expired.map((c) => card(c, true)).join("") || '<div class="faint tiny">暂无</div>') +
    "</div>" + tabbar("");
};

routes["u-guide"] = function () {
  return appHeader("用户协议与隐私政策", "u-login") +
    '<div class="app-pad"><div class="card"><b>用户协议（摘要）</b><div class="faint mt8" style="line-height:1.9">1. 本平台提供智能寄存柜服务，按网点公示价格计费；<br>2. 免费时长内取件不产生费用；<br>3. 贵重物品与违禁品不得寄存；<br>4. 柜格异常请及时通过 App 上报。</div></div>' +
    '<div class="card mt12"><b>隐私政策（摘要）</b><div class="faint mt8" style="line-height:1.9">1. 手机号仅用于登录与订单关联；<br>2. 人脸数据仅在本地演示环境模拟，不采集真实生物特征；<br>3. 订单数据用于计费与对账。</div></div></div>';
};

/* ---------- 在线客服：HTTP 轮询兜底 ---------- */
function chatBubble(m, view) {
  // view="user"（用户端）：自己=user 在右；view="agent"（客服端）：自己=agent 在右
  const mine = view === "agent" ? m.sender === "agent" : m.sender === "user";
  const who = m.sender === "system" || m.sender === "bot" ? "sys" : (mine ? "me" : "other");
  const label = who === "me" ? "我" : m.sender === "agent" ? "人工客服" : m.sender === "user" ? "用户" : "系统";
  return '<div class="chat-bubble ' + who + '"><div class="tiny faint">' + label + " · " + esc(m.time) + "</div><div class='mt5'>" + esc(m.content) + "</div></div>";
}
routes["u-chat"] = async function () {
  if (!requireLogin()) return "";
  const st = await api("/api/chat/state").catch(() => ({ state: "bot", duty: false }));
  state.chatAfter = 0;
  state._after = () => {
    pollChat();
    $("#chatSend").onclick = sendChat;
    $("#chatInput").onkeydown = (e) => { if (e.key === "Enter") sendChat(); };
    $("#toHuman").onclick = () => api("/api/chat/transfer", "POST", {}).then(() => pollChat()).catch((err) => toast(err.message));
  };
  return appHeader("在线客服", "u-home") + '<div class="app-pad">' +
    '<div class="card chat-status"><span class="chip ' + (st.duty ? "chip-ok" : "chip-warn") + '">' + (st.duty ? "人工在岗 09:00-21:00" : "人工已下班 · 工单次日处理") + '</span><span class="chip ml8">当前：' +
    (st.state === "agent" ? "人工接待中" : st.state === "waiting" ? "排队等待人工" : "智能客服") + '</span>' +
    '<button class="btn ml8" id="toHuman" style="flex:none">转人工</button></div>' +
    '<div id="chatList" class="chat-list mt12"></div>' +
    '<div class="row gap8 mt12"><input class="input flex1" id="chatInput" placeholder="输入问题…"><button class="btn btn-primary" id="chatSend">发送</button></div></div>';
};
let userChatWS = null;
function stopChatPoll() {
  if (state.chatTimer) { clearInterval(state.chatTimer); state.chatTimer = null; }
  if (userChatWS) { try { userChatWS.onclose = null; userChatWS.close(); } catch (e) {} userChatWS = null; }
}
function startChatPoll() {
  if (!state.chatTimer) state.chatTimer = setInterval(pollChat, 3000);
  // WebSocket 实时通道：收到客服消息立即拉取（失败时 HTTP 轮询仍兜底）
  // 无有效 token 时不发起连接，避免认证失败噪音
  if (!userChatWS && state.token) {
    try {
      const proto = location.protocol === "https:" ? "wss:" : "ws:";
      userChatWS = new WebSocket(proto + "//" + location.host + "/api/ws/chat?token=" + encodeURIComponent(state.token || ""));
      userChatWS.onmessage = (ev) => {
        try { const m = JSON.parse(ev.data); if (m.type === "message") pollChat(); } catch (e) {}
      };
      userChatWS.onclose = () => { userChatWS = null; };
    } catch (e) { userChatWS = null; }
  }
}
async function pollChat() {
  const list = $("#chatList");
  if (!list) return;
  const data = await api("/api/chat/messages?afterId=" + state.chatAfter).catch(() => null);
  if (!data) return;
  if (data.messages && data.messages.length) {
    state.chatAfter = data.messages[data.messages.length - 1].id;
    list.innerHTML += data.messages.map((m) => chatBubble(m, "user")).join("");
    list.scrollTop = list.scrollHeight;
  }
  const stc = document.querySelector(".chat-status .chip:last-of-type");
  if (stc) stc.textContent = "当前：" + (data.state === "agent" ? "人工接待中" : data.state === "waiting" ? "排队等待人工" : "智能客服");
  const th = $("#toHuman");
  if (th) {
    th.disabled = data.state === "agent" || data.state === "waiting";
    th.textContent = data.state === "agent" ? "人工接待中" : data.state === "waiting" ? "排队等待中" : "转人工";
  }
}
function sendChat() {
  const input = $("#chatInput");
  const content = input.value.trim();
  if (!content) return;
  input.value = "";
  api("/api/chat/send", "POST", { content }).then((r) => {
    if (r.ticket) toast("已生成工单 " + r.ticket);
    pollChat();
  }).catch((err) => toast(err.message));
}

/* ================================================================
 *  柜机端（Kiosk 大屏）
 * ================================================================ */
function kClock() {
  return new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
}
routes["k-standby"] = async function () {
  const cities = await api("/api/cities").catch(() => []);
  const sh = cities.find((c) => c.city === state.city) || cities[0] || { idle: 0 };
  const detail = await api("/api/outlets?city=" + encodeURIComponent(state.city)).then((l) => l[0]).catch(() => null);
  const cabDetail = detail ? await api("/api/outlets/" + detail.id).catch(() => null) : null;
  const cabinets = (cabDetail && cabDetail.cabinets) || [];
  state._after = () => { if (state.kTimer) clearInterval(state.kTimer); state.kTimer = setInterval(() => { const el = $("#kClock"); if (el) el.textContent = kClock(); }, 30000); };
  return '<div class="kiosk kiosk-idle">' +
    '<div class="k-brand"><span class="k-brand-m">◧</span> 优小宝寄存<span class="k-clock" id="kClock">● ' + kClock() + '</span></div>' +
    '<div class="k-center"><span class="k-eyebrow">SMART LOCKER · 智能寄存</span>' +
    '<h1>安心寄存<br><em>轻松取件</em></h1><p>' + esc(state.city) + ' · 人民广场站 2 号口 · 当前 <b>' + sh.idle + '</b> 格空闲</p>' +
    '<div class="k-actions"><button class="k-btn" data-action="kioskStore"><span class="ki">↓</span><span><b class="n">存 件</b><span class="d">扫码下单 · 即存即走</span></span><span class="ar">→</span></button>' +
    '<button class="k-btn k-btn-alt" data-action="kioskPickup"><span class="ki">↑</span><span><b class="n">取 件</b><span class="d">输入取件码 · 开柜</span></span><span class="ar">→</span></button></div></div>' +
    '<div class="k-art"><div class="hd">柜格实时状态 LIVE</div><div class="grid">' +
    (cabinets.length ? cabinets.slice(0, 24).map((c) => '<div class="d ' + (c.status === "occupied" ? "used" : c.status === "fault" ? "err" : "") + '" title="' + esc(c.code) + ' ' + statusName[c.status] + '">' + esc(c.code.replace(/^[A-Za-z]-?/, "")) + '</div>').join("") : '<div class="faint tiny">暂无数据</div>') + '</div></div>' +
    '<div class="k-foot"><span>24 小时自助服务</span><span>收费透明 · 多退少补</span><span class="live">● 系统在线</span></div></div>';
};

routes["k-store"] = async function () {
  const outlets = await api("/api/outlets?city=" + encodeURIComponent(state.city)).catch(() => []);
  const first = outlets[0];
  let cellsHtml = '<div class="faint tiny">暂无柜格数据</div>';
  if (first) {
    // 使用公开接口 /api/outlets/{id} 的柜格统计，不再依赖后台 staff 权限接口
    const detail = await api("/api/outlets/" + first.id).catch(() => null);
    if (detail && detail.byStatus) {
      const s = detail.byStatus;
      const cells = [];
      for (let i = 0; i < (s.idle || 0); i++) cells.push('<div class="lg-cell" title="空闲"></div>');
      for (let i = 0; i < (s.occupied || 0); i++) cells.push('<div class="lg-cell used" title="占用"></div>');
      for (let i = 0; i < (s.fault || 0); i++) cells.push('<div class="lg-cell err" title="异常"></div>');
      if (cells.length) cellsHtml = cells.join("");
    }
  }
  const sizes = [["S", "小格", "5 元/时", "适合背包、手提袋"], ["M", "中格", "7 元/时", "适合行李箱、电脑包"], ["L", "大格", "10 元/时", "适合大件行李"]];
  return '<div class="kiosk"><div class="k-head"><button class="icon-btn" data-action="kioskHome">' + I.back + '</button><div class="ttl">存件 · 选择柜型</div><span class="ml-auto tiny faint">10.1 寸触屏</span></div>' +
    '<div class="k-pad"><div class="h2">请选择柜格规格</div><div class="faint tiny mt5">计时从关门后开始 · 免费 10 分钟 · 24h 封顶</div>' +
    '<div class="k-spec-row mt16">' + sizes.map(([k, n, p, d]) =>
      '<button class="k-spec" data-size="' + k + '"><span class="tick">✓</span><div class="pic">' + (k === "S" ? "▣" : k === "M" ? "▤" : "▥") + '</div><b>' + n + '</b><div class="sub">' + d + '</div><div class="p">' + p + '</div></button>').join("") + '</div>' +
    '<div class="k-pay-qr mt16"><div><b class="h2">扫码支付下单</b><div class="faint tiny mt5">用手机扫描二维码，在用户端完成选柜与支付；支付成功后柜门自动弹开。</div>' +
    '<div class="row gap8 mt12"><button class="btn btn-primary" data-action="kioskTip">生成演示二维码</button><button class="btn" data-action="kioskHome">返回待机</button></div></div>' +
    '<div class="qr-placeholder"><div class="mono">QR</div><span>扫码下单</span></div></div>' +
    '<div class="section-h mt16"><b>本站柜格实时状态</b></div><div id="k-cells" class="locker-grid-mini mt10">' + cellsHtml + '</div></div></div>';
};

routes["k-pickup"] = function () {
  state.kCode = "";
  const renderCode = () => {
    const boxes = $("#kCodeBoxes");
    if (boxes) boxes.innerHTML = Array.from({ length: 6 }, (_, i) => '<div class="cb">' + (state.kCode[i] || "") + '</div>').join("");
  };
  state._after = () => {
    renderCode();
    document.querySelectorAll(".k-key").forEach((b) => {
      b.onclick = () => {
        const k = b.dataset.k;
        if (k === "del") state.kCode = state.kCode.slice(0, -1);
        else if (k === "ok") { doKPickup(); return; }
        else if (state.kCode.length < 6) state.kCode += k;
        renderCode();
      };
    });
  };
  return '<div class="kiosk"><div class="k-head"><button class="icon-btn" data-action="kioskHome">' + I.back + '</button><div class="ttl">取件 · 输入取件码</div><span class="ml-auto tiny faint">10.1 寸触屏</span></div>' +
    '<div class="k-pad k-pickup-pad"><div class="k-pickup-main"><div class="h2">请输入 6 位取件码</div><div class="faint tiny mt5">取件码在下单成功页与「我的订单」中查看</div>' +
    '<div class="code-boxes" id="kCodeBoxes"></div><div id="kMsg" class="mt12"></div></div>' +
    '<div class="keypad">' +
    ["1", "2", "3", "4", "5", "6", "7", "8", "9", "del", "0", "ok"].map((k) =>
      '<button class="k-key ' + (k === "del" ? "del" : k === "ok" ? "go" : "") + '" data-k="' + k + '">' + (k === "del" ? "删除" : k === "ok" ? "开柜" : k) + '</button>').join("") +
    '</div></div></div>';
};

async function doKPickup() {
  const code = state.kCode || "";
  const box = $("#kMsg");
  if (code.length !== 6) { if (box) box.innerHTML = '<div class="k-err">请输入完整 6 位取件码</div>'; return; }
  if (box) box.innerHTML = "";
  try {
    const r = await api("/api/kiosk/pickup", "POST", { code });
    if (box) box.innerHTML = '<div class="card"><div class="big-ok">柜门已弹开 · 柜格 ' + esc(r.lockerCode) + '</div><button class="btn btn-primary mt12" id="kDone" style="width:100%">已取出物品，关门结算</button></div>';
    $("#kDone").onclick = async () => {
      try {
        const s = await api("/api/kiosk/orders/" + r.orderId + "/pickup-done", "POST", {});
        if (box) box.innerHTML = Number(s.extra) > 0
          ? '<div class="k-err">取件完成，实际费用超出预付 ¥' + yuan2(s.extra) + '，请在用户端订单中完成补交后订单才会结束</div>'
          : '<div class="big-ok">✓ 取件完成 · ' + (s.kind === "free" ? "免费时长内，已全额退款" : "结算完成，订单已结束") + '</div>';
      } catch (err) { if (box) box.innerHTML = '<div class="k-err">结算失败：' + esc(err.message) + '</div>'; }
    };
  } catch (err) {
    if (box) box.innerHTML = '<div class="k-err">✕ ' + esc(err.message) + '</div>';
    state.kCode = "";
    const cb = $("#kCodeBoxes"); if (cb) cb.innerHTML = Array.from({ length: 6 }, () => '<div class="cb"></div>').join("");
  }
}

/* ================================================================
 *  管理后台
 * ================================================================ */
const adminMenus = [
  ["m-dash", "dashboard", "数据看板"], ["m-outlets", "outlets", "网点管理"], ["m-monitor", "monitor", "柜格监控"], ["m-rules", "rules", "收费规则"],
  ["m-coupons", "coupons", "优惠券"], ["m-orders", "orders", "订单管理"], ["m-revenue", "revenue", "营收明细"], ["m-alerts", "alerts", "异常告警"],
  ["m-staff", "staff", "员工管理"], ["m-cs", "cs", "客服工作台"],
];
const MENU_KEY = Object.fromEntries(adminMenus.map(([view, key]) => [view, key]));
// 后端返回菜单 id（dashboard/outlets/...），统一转换为前端视图 id（m-dash/...）
function allowedViews(info) {
  const raw = (info && info.menus) || [];
  const mapped = adminMenus.filter(([, key]) => raw.includes(key)).map(([view]) => view);
  return raw.includes("m-dash") || raw.some((x) => x.startsWith("m-")) ? raw : mapped;
}
function adminShell(active, body) {
  const info = state.adminInfo || JSON.parse(localStorage.getItem("locker_admin") || "null") || { name: "管理员", role: "", menus: [] };
  const allowed = allowedViews(info);
  return '<div class="admin-wrap"><div class="admin-side">' +
    '<div class="admin-logo">Smart-Locker 后台</div>' +
    '<div class="tiny faint" style="padding:0 16px">' + esc(info.name) + (info.role ? " · " + roleCn(info.role) : "") + '</div>' +
    adminMenus.filter((m) => allowed.includes(m[0])).map((m) =>
      '<button class="admin-nav ' + (active === m[0] ? "on" : "") + '" data-action="adminNav" data-id="' + m[0] + '">' + esc(m[2]) + "</button>").join("") +
    '<button class="admin-nav" data-action="adminLogout" style="margin-top:auto">退出登录</button></div>' +
    '<div class="admin-main">' + body + "</div></div>";
}
function roleCn(r) { return { super: "超级管理员", site_admin: "网点管理员", service: "在线客服" }[r] || r; }

routes["m-login"] = function () {
  return '<div class="admin-wrap"><div class="admin-login"><div class="admin-logo">Smart-Locker 管理后台</div>' +
    '<input class="input mt16" id="admId" placeholder="账号或手机号">' +
    '<input class="input mt12" id="admPwd" type="password" placeholder="密码">' +
    '<button class="btn btn-primary mt16" id="admLogin" style="width:100%">登 录</button></div></div>';
};
document.addEventListener("click", (e) => {
  if (e.target.closest("#admLogin")) {
    const identity = $("#admId").value.trim();
    const password = $("#admPwd").value;
    api("/api/admin/login", "POST", { identity, password }).then((r) => {
      localStorage.setItem("locker_admin", JSON.stringify(r));
      state.adminToken = r.token;
      state.adminInfo = r;
      go("m-dash");
    }).catch((err) => {
      toast(err.message, 3500);
      const box = $("#admLoginErr");
      if (box) { box.textContent = "✕ " + err.message; box.style.display = "block"; }
      else { const n = document.createElement("div"); n.id = "admLoginErr"; n.className = "login-err"; n.textContent = "✕ " + err.message; $("#admLogin").before(n); }
    });
  }
});

/* 管理端 API 走独立 token */
async function admApi(path, method, body) {
  const info = JSON.parse(localStorage.getItem("locker_admin") || "null");
  const headers = { "Content-Type": "application/json" };
  if (info && info.token) headers["X-Token"] = info.token;
  const resp = await fetch(path, { method: method || "GET", headers, body: body ? JSON.stringify(body) : undefined });
  let data = null;
  try { data = await resp.json(); } catch (e) {}
  if (!resp.ok) throw new Error((data && data.error) || ("HTTP " + resp.status));
  return data;
}

function adminGuard(view) {
  const raw = localStorage.getItem("locker_admin");
  if (!raw) { go("m-login"); return ""; }
  const info = JSON.parse(raw);
  if (!allowedViews(info).includes(view)) return adminShell("m-dash", '<div class="card">无权限访问该模块</div>');
  return null;
}

routes["m-dash"] = async function () {
  const g = adminGuard("m-dash");
  if (g !== null) return g;
  const d = await admApi("/api/admin/dashboard?range=" + (state.dashRange || "today"));
  state._after = () => {
    document.querySelectorAll(".dash-r").forEach((b) => { b.onclick = () => { state.dashRange = b.dataset.r; render(); }; });
  };
  return adminShell("m-dash",
    '<div class="row" style="justify-content:space-between"><div class="h2">数据看板</div><div>' +
    ["today|今日", "yesterday|昨日", "month|本月"].map((x) => { const [k, n] = x.split("|"); return '<button class="btn ' + (state.dashRange === k ? "btn-primary" : "") + ' dash-r" data-r="' + k + '">' + n + "</button>"; }).join(" ") + "</div></div>" +
    '<div class="stat-grid mt16">' +
    statCard("净营收（收款-退款+补交）", "¥" + yuan2(d.revenue)) + statCard("订单数", d.orders) + statCard("柜格使用率", d.usageRate.toFixed(1) + "%") + statCard("未处理告警", d.openAlerts) +
    "</div>" +
    '<div class="card mt16"><div class="bold14 mb10">近 7 日订单趋势</div><div class="trend-row">' +
    d.trend.map((t) => '<div class="trend-col"><div class="mono bold14">' + t.orders + '</div><div class="trend-bar" style="height:' + (8 + Math.min(t.orders, 20) * 5) + 'px"></div><div class="tiny faint">' + esc(t.date) + "</div></div>").join("") +
    "</div></div>" +
    '<div class="card mt16 faint tiny">收款 ¥' + yuan2(d.income) + " · 退款 ¥" + yuan2(d.refund) + " · 补交 ¥" + yuan2(d.extra) + " · 使用柜格 " + d.usedLockers + "/" + d.totalLockers + "</div>");
};
function statCard(lbl, val) {
  return '<div class="stat-card"><div class="tiny faint">' + lbl + '</div><div class="stat-val mono">' + esc(val) + "</div></div>";
}

routes["m-outlets"] = async function () {
  const g = adminGuard("m-outlets");
  if (g !== null) return g;
  const outs = await admApi("/api/admin/outlets").catch(() => []);
  state._after = () => {
    $("#addOutlet").onclick = () => {
      modal('<div class="bold14 mb10">新增网点</div><input class="input" id="noName" placeholder="网点名称"><input class="input mt8" id="noAddr" placeholder="地址"><input class="input mt8" id="noCity" placeholder="城市"><button class="btn btn-primary mt12" id="noSave" style="width:100%">创建（自动配柜组与默认规则）</button>');
      $("#noSave").onclick = async () => {
        try {
          await admApi("/api/admin/outlets", "POST", { name: $("#noName").value, address: $("#noAddr").value, city: $("#noCity").value, status: "normal" });
          closeModal(); toast("创建成功"); render();
        } catch (err) { toast(err.message); }
      };
    };
    document.querySelectorAll(".out-toggle").forEach((b) => {
      b.onclick = () => {
        const st = b.dataset.st === "normal" ? "paused" : "normal";
        admApi("/api/admin/outlets/" + b.dataset.id, "PUT", { status: st }).then(() => { toast("已更新"); render(); }).catch((err) => toast(err.message));
      };
    });
  };
  return adminShell("m-outlets",
    '<div class="row" style="justify-content:space-between"><div class="h2">网点管理</div><button class="btn btn-primary" id="addOutlet">新增网点</button></div>' +
    '<table class="tbl mt16"><thead><tr><th>ID</th><th>名称</th><th>城市</th><th>地址</th><th>状态</th><th>空闲</th><th>操作</th></tr></thead><tbody>' +
    outs.map((o) => "<tr><td>" + o.id + "</td><td>" + esc(o.name) + "</td><td>" + esc(o.city) + "</td><td>" + esc(o.address) + '</td><td>' +
      (o.status === "normal" ? '<span class="chip chip-ok">营业</span>' : '<span class="chip chip-err">暂停</span>') + "</td><td class='mono'>" + o.idle + "</td><td>" +
      '<button class="btn sm out-toggle" data-id="' + o.id + '" data-st="' + o.status + '">' + (o.status === "normal" ? "暂停" : "恢复") + "</button></td></tr>").join("") +
    "</tbody></table>");
};

routes["m-monitor"] = async function () {
  const g = adminGuard("m-monitor");
  if (g !== null) return g;
  const me = JSON.parse(localStorage.getItem("locker_admin") || "null") || {};
  // 网点管理员无权访问"网点管理"模块，使用登录态里的 scopeOutletId，不调 /api/admin/outlets
  const scope = me.scopeOutletId || 0;
  const outs = me.role === "site_admin"
    ? [{ id: scope, name: "本网点" }]
    : await admApi("/api/admin/outlets").catch(() => []);
  const oid = state.monOutlet || (scope && me.role === "site_admin" ? scope : (outs[0] && outs[0].id)) || 0;
  const lockers = oid ? await admApi("/api/admin/lockers?outletId=" + oid).catch(() => []) : [];
  state._after = () => {
    document.querySelectorAll(".mon-o").forEach((b) => { b.onclick = () => { state.monOutlet = Number(b.dataset.id); render(); }; });
    document.querySelectorAll(".mon-repair").forEach((b) => {
      b.onclick = () => {
        admApi("/api/admin/lockers/" + b.dataset.lid + "/action", "POST", { action: "repair" }).then(() => { toast("已派单维修并恢复柜格"); render(); }).catch((err) => toast(err.message));
      };
    });
    document.querySelectorAll(".cab-cell[data-lid]").forEach((c) => {
      c.onclick = async () => {
        const lid = Number(c.dataset.lid);
        const d = await admApi("/api/admin/lockers/" + lid).catch(() => null);
        if (!d) return;
        modal('<div class="bold14 mb10">柜格 ' + esc(d.code) + " " + statusChip(d.status) + "</div>" +
          '<div class="tiny faint">状态：' + statusName[d.status] + (d.faultSource ? " · 来源：" + esc(d.faultSource) : "") + (d.faultNote ? " · " + esc(d.faultNote) : "") + "</div>" +
          (d.order ? '<div class="tiny faint mt8">关联订单 ' + esc(d.order.orderNo) + "</div>" : "") +
          '<div class="row gap8 mt16"><button class="btn flex1 mon-act" data-a="remote_open">远程开柜</button><button class="btn flex1 mon-act" data-a="repair">报修恢复</button><button class="btn flex1 mon-act" data-a="switch">切换柜格</button></div>');
        document.querySelectorAll(".mon-act").forEach((b) => {
          b.onclick = () => {
            admApi("/api/admin/lockers/" + lid + "/action", "POST", { action: b.dataset.a }).then((r) => {
              closeModal(); toast(r.note || ("新柜格 " + r.newLocker)); render();
            }).catch((err) => toast(err.message));
          };
        });
      };
    });
  };
  const bs = { idle: 0, occupied: 0, fault: 0 };
  lockers.forEach((l) => { if (bs[l.status] !== undefined) bs[l.status]++; });
  const faults = lockers.filter((l) => l.status === "fault");
  return adminShell("m-monitor",
    '<div class="h2">柜格实时监控</div><div class="faint tiny mt5">按柜格编号查看实时状态，点击柜格可执行 远程开柜 / 报修恢复 / 切换柜格</div>' +
    '<div class="row gap8 mt12">' + outs.map((o) => '<button class="btn ' + (o.id === oid ? "btn-primary" : "") + ' mon-o" data-id="' + o.id + '">' + esc(o.name) + "</button>").join("") + '</div>' +
    '<div class="card mt16"><div class="row" style="justify-content:space-between"><b>柜格状态网格</b><span class="faint tiny">共 ' + lockers.length + ' 格</span></div>' +
    '<div class="mt12" id="mon-cells">' + cabinetGrid(lockers.map((l) => ({ code: l.code, status: l.status, id: l.id })), true) + '</div>' +
    cabinetLegend(bs) + '</div>' +
    (faults.length ? '<div class="card mt12"><b>异常柜格（' + faults.length + '）</b><table class="tbl mt10"><thead><tr><th>柜格</th><th>来源</th><th>问题</th><th>操作</th></tr></thead><tbody>' +
      faults.map((l) => '<tr><td class="mono">' + esc(l.code) + '</td><td>' + esc(l.faultSource || "系统自检") + '</td><td>' + esc(l.faultNote || "传感器异常") + '</td><td><button class="btn sm mon-repair" data-lid="' + l.id + '">派单维修</button></td></tr>').join("") + '</tbody></table></div>' : ''));
};

routes["m-rules"] = async function () {
  const g = adminGuard("m-rules");
  if (g !== null) return g;
  const outs = await admApi("/api/admin/outlets").catch(() => []);
  const oid = state.ruleOutlet || (outs[0] && outs[0].id) || 0;
  const rule = oid ? await admApi("/api/admin/rules/" + oid).catch(() => null) : null;
  const f = (id, lbl, val) => '<label class="lbl">' + lbl + '</label><input class="input" id="' + id + '" value="' + (val == null ? "" : esc(val)) + '">';
  state._after = () => {
    document.querySelectorAll(".rule-o").forEach((b) => { b.onclick = () => { state.ruleOutlet = Number(b.dataset.id); render(); }; });
    const save = $("#saveRule");
    if (save) save.onclick = async () => {
      try {
        await admApi("/api/admin/rules/" + oid, "PUT", {
          mode: $("#rMode").value, freeMinutes: Number($("#rFree").value),
          priceS: Number($("#rPS").value), priceM: Number($("#rPM").value), priceL: Number($("#rPL").value),
          depositS: Number($("#rDS").value), depositM: Number($("#rDM").value), depositL: Number($("#rDL").value),
          capS: Number($("#rCS").value), capM: Number($("#rCM").value), capL: Number($("#rCL").value),
        });
        toast("已保存");
      } catch (err) { toast(err.message); }
    };
  };
  return adminShell("m-rules",
    '<div class="h2">收费规则</div><div class="row gap8 mt12">' +
    outs.map((o) => '<button class="btn ' + (o.id === oid ? "btn-primary" : "") + ' rule-o" data-id="' + o.id + '">' + esc(o.name) + "</button>").join("") + "</div>" +
    (rule ? '<div class="card mt16">' +
      '<div class="row gap16"><div style="flex:1">' + f("rMode", "收费模式（hourly=按时 daily=按日）", rule.mode) + "</div><div style='flex:1'>" + f("rFree", "免费时长(分钟)", rule.freeMinutes) + "</div></div>" +
      '<div class="row gap16 mt12"><div style="flex:1">' + f("rPS", "小格单价(分)", rule.priceS) + "</div><div style='flex:1'>" + f("rPM", "中格单价(分)", rule.priceM) + "</div><div style='flex:1'>" + f("rPL", "大格单价(分)", rule.priceL) + "</div></div>" +
      '<div class="row gap16 mt12"><div style="flex:1">' + f("rDS", "小格押金", rule.depositS) + "</div><div style='flex:1'>" + f("rDM", "中格押金", rule.depositM) + "</div><div style='flex:1'>" + f("rDL", "大格押金", rule.depositL) + "</div></div>" +
      '<div class="row gap16 mt12"><div style="flex:1">' + f("rCS", "小格封顶", rule.capS) + "</div><div style='flex:1'>" + f("rCM", "中格封顶", rule.capM) + "</div><div style='flex:1'>" + f("rCL", "大格封顶", rule.capL) + "</div></div>" +
      '<button class="btn btn-primary mt16" id="saveRule" style="width:100%">保存规则</button></div>' : '<div class="card mt16 faint">该网点未配置规则</div>'));
};

routes["m-coupons"] = async function () {
  const g = adminGuard("m-coupons");
  if (g !== null) return g;
  const defs = await admApi("/api/admin/coupons").catch(() => []);
  state._after = () => {
    $("#addCp").onclick = () => {
      const today = new Date().toISOString().slice(0, 10);
      const nextYear = (Number(today.slice(0, 4)) + 1) + today.slice(4);
      modal('<div class="bold14 mb10">新建优惠券</div>' +
        '<input class="input" id="cpName" placeholder="券名称（必填）">' +
        '<div class="row gap8 mt8"><select class="input flex1" id="cpType"><option value="discount">折扣券</option><option value="hour">时长券(分钟)</option><option value="amount">金额券(元)</option></select>' +
        '<input class="input flex1" id="cpVal" value="80" placeholder="面值"></div>' +
        '<div class="faint tiny mt5" id="cpHint">折扣券填 80 表示 8 折（1-99）；时长券填分钟数；金额券填元</div>' +
        '<div class="row gap8 mt8"><input class="input flex1" id="cpFrom" type="date" value="' + today + '"><input class="input flex1" id="cpTo" type="date" value="' + nextYear + '"><input class="input flex1" id="cpTotal" type="number" min="1" value="100" placeholder="数量"></div>' +
        '<button class="btn btn-primary mt12" id="cpSave" style="width:100%">创建</button>');
      $("#cpType").onchange = () => {
        const t = $("#cpType").value;
        $("#cpVal").value = t === "discount" ? "80" : t === "hour" ? "60" : "1";
        $("#cpHint").textContent = t === "discount" ? "折扣券：填 80 表示 8 折（1-99）" : t === "hour" ? "时长券：填分钟数，如 60 表示 1 小时" : "金额券：填元，如 1 表示立减 1 元";
      };
      $("#cpSave").onclick = async () => {
        const name = $("#cpName").value.trim();
        const type = $("#cpType").value;
        const valRaw = Number($("#cpVal").value);
        // 金额券界面按元输入，后端按分存储
        const value = type === "amount" ? Math.round(valRaw * 100) : valRaw;
        if (!name) { toast("请填写券名称"); return; }
        if (!valRaw || valRaw <= 0) { toast("请填写有效面值"); return; }
        if (type === "discount" && (valRaw < 1 || valRaw >= 100)) { toast("折扣券填 1-99，如 80 表示8折"); return; }
        try {
          await admApi("/api/admin/coupons", "POST", {
            name, type, value, scope: "all",
            validFrom: $("#cpFrom").value, validTo: $("#cpTo").value, totalCount: Number($("#cpTotal").value) || 100,
          });
          closeModal(); toast("优惠券创建成功"); render();
        } catch (err) { toast(err.message); }
      };
    };
    document.querySelectorAll(".cp-toggle").forEach((b) => {
      b.onclick = () => {
        admApi("/api/admin/coupons/" + b.dataset.id + "/toggle", "POST", { enabled: b.dataset.en !== "true" }).then(() => { toast("已更新"); render(); }).catch((err) => toast(err.message));
      };
    });
  };
  return adminShell("m-coupons",
    '<div class="row" style="justify-content:space-between"><div class="h2">优惠券</div><button class="btn btn-primary" id="addCp">新建优惠券</button></div>' +
    '<table class="tbl mt16"><thead><tr><th>ID</th><th>名称</th><th>类型</th><th>面额/折扣</th><th>范围</th><th>有效期</th><th>发放/已领</th><th>状态</th><th>操作</th></tr></thead><tbody>' +
    defs.map((d) => "<tr><td>" + d.id + "</td><td>" + esc(d.name) + "</td><td>" + ({ discount: "折扣券", hour: "时长券", amount: "金额券" }[d.type] || d.type) + "</td><td class='mono'>" +
      (d.type === "discount" ? (d.value / 10) + "折" : d.type === "hour" ? d.value + "分钟" : "¥" + yuan(d.value)) + "</td><td>" + (d.scope === "outlet" ? "网点" + d.outletId : "全部") + "</td><td class='tiny'>" + esc(d.validFrom) + "~" + esc(d.validTo) + "</td><td class='mono'>" + d.used + "/" + d.total + "</td><td>" +
      (d.enabled ? '<span class="chip chip-ok">启用</span>' : '<span class="chip chip-err">停用</span>') + "</td><td>" +
      '<button class="btn sm cp-toggle" data-id="' + d.id + '" data-en="' + d.enabled + '">' + (d.enabled ? "停用" : "启用") + "</button></td></tr>").join("") +
    "</tbody></table>");
};

routes["m-orders"] = async function () {
  const g = adminGuard("m-orders");
  if (g !== null) return g;
  const st = state.admOrderStatus || "";
  const orders = await admApi("/api/admin/orders" + (st ? "?status=" + st : "")).catch(() => []);
  state._after = () => {
    document.querySelectorAll(".ord-f").forEach((b) => { b.onclick = () => { state.admOrderStatus = b.dataset.k; render(); }; });
  };
  return adminShell("m-orders",
    '<div class="h2">订单管理</div><div class="row gap8 mt12">' +
    ["|全部", "active|进行中", "finished|已结束"].map((x) => { const [k, n] = x.split("|"); return '<button class="btn ' + (st === k ? "btn-primary" : "") + ' ord-f" data-k="' + k + '">' + n + "</button>"; }).join(" ") + "</div>" +
    '<table class="tbl mt16"><thead><tr><th>订单号</th><th>网点</th><th>柜型</th><th>模式</th><th>状态</th><th>费用</th><th>退款</th><th>补交</th><th>取件方式</th></tr></thead><tbody>' +
    orders.map((o) => "<tr><td class='mono tiny'>" + esc(o.orderNo) + "</td><td>" + o.outletId + "</td><td>" + (sizeName[o.size] || o.size || "") + "</td><td>" + (o.payMode === "deposit" ? "押金" : "预付") + "</td><td>" +
      (o.status === "active" ? '<span class="chip chip-ok">进行中</span>' : '<span class="chip">已结束</span>') + "</td><td class='mono'>¥" + yuan2(o.fee) + "</td><td class='mono'>¥" + yuan2(o.refund) + "</td><td class='mono'>" + (o.extra ? "¥" + yuan2(o.extra) + (o.extraPaid ? "✓" : "未付") : "-") + "</td><td>" + (o.pickupMethod || "-") + "</td></tr>").join("") +
    "</tbody></table>");
};

routes["m-revenue"] = async function () {
  const g = adminGuard("m-revenue");
  if (g !== null) return g;
  const dim = state.revDim || "outlet";
  const r = await admApi("/api/admin/revenue?dim=" + dim).catch(() => ({ rows: [] }));
  state._after = () => {
    document.querySelectorAll(".rev-d").forEach((b) => { b.onclick = () => { state.revDim = b.dataset.k; render(); }; });
    $("#expCsv").onclick = () => {
      const info = JSON.parse(localStorage.getItem("locker_admin"));
      window.open("/api/admin/revenue/export?token=" + info.token);
    };
  };
  return adminShell("m-revenue",
    '<div class="row" style="justify-content:space-between"><div class="h2">营收明细（本月）</div><button class="btn" id="expCsv">导出对账单 CSV</button></div>' +
    '<div class="row gap8 mt12">' + ["outlet|按网点", "group|按柜组", "order|按订单"].map((x) => { const [k, n] = x.split("|"); return '<button class="btn ' + (dim === k ? "btn-primary" : "") + ' rev-d" data-k="' + k + '">' + n + "</button>"; }).join(" ") + "</div>" +
    '<div class="stat-grid mt16">' + statCard("收款", "¥" + yuan2(r.income)) + statCard("退款", "¥" + yuan2(r.refund)) + statCard("补交", "¥" + yuan2(r.extra)) + statCard("净营收", "¥" + yuan2(r.total)) + "</div>" +
    '<table class="tbl mt16"><thead><tr>' +
    (dim === "outlet" ? "<th>网点</th><th>收款</th><th>退款</th><th>补交</th><th>净额</th>" :
      dim === "group" ? "<th>柜组</th><th>收款</th><th>退款</th><th>补交</th><th>净额</th>" :
        "<th>订单</th><th>类型</th><th>金额</th><th>时间</th>") + "</tr></thead><tbody>" +
    (r.rows || []).map((row) => dim === "order" ?
      "<tr><td class='mono'>" + row.orderId + "</td><td>" + ({ income: "收款", refund: "退款", extra: "补交" }[row.kind] || row.kind) + "</td><td class='mono'>¥" + yuan2(row.amount) + "</td><td class='tiny'>" + esc(row.time) + "</td></tr>" :
      "<tr><td>" + esc(row.name || ("#" + (row.outletId || row.groupId))) + "</td><td class='mono'>¥" + yuan2(row.income) + "</td><td class='mono'>¥" + yuan2(row.refund) + "</td><td class='mono'>¥" + yuan2(row.extra) + "</td><td class='mono'><b>¥" + yuan2(row.net) + "</b></td></tr>").join("") +
    "</tbody></table>");
};

routes["m-alerts"] = async function () {
  const g = adminGuard("m-alerts");
  if (g !== null) return g;
  const st = state.alertStatus || "open";
  const alerts = await admApi("/api/admin/alerts?status=" + st).catch(() => ({ alerts: [] }));
  const list = alerts.alerts || [];
  state._after = () => {
    document.querySelectorAll(".al-f").forEach((b) => { b.onclick = () => { state.alertStatus = b.dataset.k; render(); }; });
    document.querySelectorAll(".al-handle").forEach((b) => {
      b.onclick = () => {
        admApi("/api/admin/alerts/" + b.dataset.id + "/handle", "POST", {}).then(() => { toast("已处理"); render(); }).catch((err) => toast(err.message));
      };
    });
  };
  return adminShell("m-alerts",
    '<div class="h2">异常告警</div><div class="row gap8 mt12">' +
    ["open|未处理", "handled|已处理", "|全部"].map((x) => { const [k, n] = x.split("|"); return '<button class="btn ' + (st === k ? "btn-primary" : "") + ' al-f" data-k="' + k + '">' + n + "</button>"; }).join(" ") + "</div>" +
    '<div class="mt16">' + (list.length ? list.map((a) =>
      '<div class="card mb10"><div class="row" style="justify-content:space-between"><b>' + esc(a.title) + '</b><span class="chip ' + (a.level === "high" ? "chip-err" : "chip-warn") + '">' + (a.level === "high" ? "高" : "中") + "</span></div>" +
      '<div class="tiny faint mt5">' + esc(a.description) + " · " + esc(a.time) + " · 来源 " + esc(a.source) + (a.handler ? " · 处理人 " + esc(a.handler) : "") + "</div>" +
      (a.status === "open" ? '<button class="btn sm mt8 al-handle" data-id="' + a.id + '">标记已处理</button>' : '<span class="chip chip-ok mt8" style="display:inline-block">已处理</span>') + "</div>").join("")
      : '<div class="card faint">暂无告警</div>') + "</div>");
};

routes["m-staff"] = async function () {
  const g = adminGuard("m-staff");
  if (g !== null) return g;
  const staffs = await admApi("/api/admin/staff").catch(() => []);
  state._after = () => {
    $("#addStaff").onclick = () => {
      modal('<div class="bold14 mb10">新建员工</div>' +
        '<div class="row gap8"><input class="input flex1" id="sfNo" placeholder="工号"><input class="input flex1" id="sfAcc" placeholder="账号"></div>' +
        '<div class="row gap8 mt8"><input class="input flex1" id="sfName" placeholder="姓名"><input class="input flex1" id="sfPhone" placeholder="手机号"></div>' +
        '<div class="row gap8 mt8"><select class="input flex1" id="sfRole"><option value="super">超级管理员</option><option value="site_admin">网点管理员</option><option value="service">在线客服</option></select><input class="input flex1" id="sfOutlet" placeholder="网点ID(网点管理员填)"></div>' +
        '<input class="input mt8" id="sfPwd" type="password" placeholder="初始密码">' +
        '<button class="btn btn-primary mt12" id="sfSave" style="width:100%">创建</button>');
      $("#sfSave").onclick = async () => {
        try {
          await admApi("/api/admin/staff", "POST", {
            empNo: $("#sfNo").value, account: $("#sfAcc").value, name: $("#sfName").value, phone: $("#sfPhone").value,
            role: $("#sfRole").value, password: $("#sfPwd").value, scopeOutletId: Number($("#sfOutlet").value || 0),
          });
          closeModal(); toast("创建成功"); render();
        } catch (err) { toast(err.message); }
      };
    };
    document.querySelectorAll(".st-role").forEach((b) => {
      b.onclick = () => {
        const nid = prompt("输入新角色：super / site_admin / service");
        if (!nid) return;
        admApi("/api/admin/staff/" + b.dataset.id + "/role", "POST", { role: nid }).then(() => { toast("已修改，登录态已失效"); render(); }).catch((err) => toast(err.message));
      };
    });
    document.querySelectorAll(".st-pwd").forEach((b) => {
      b.onclick = () => {
        const pwd = prompt("输入新密码（≥6位）");
        if (!pwd) return;
        admApi("/api/admin/staff/" + b.dataset.id + "/reset-pwd", "POST", { password: pwd }).then(() => { toast("已重置"); }).catch((err) => toast(err.message));
      };
    });
    document.querySelectorAll(".st-toggle").forEach((b) => {
      b.onclick = () => {
        admApi("/api/admin/staff/" + b.dataset.id + "/toggle", "POST", { status: b.dataset.st === "active" ? "disabled" : "active" }).then(() => { toast("已更新"); render(); }).catch((err) => toast(err.message));
      };
    });
  };
  return adminShell("m-staff",
    '<div class="row" style="justify-content:space-between"><div class="h2">员工管理</div><button class="btn btn-primary" id="addStaff">新建员工</button></div>' +
    '<table class="tbl mt16"><thead><tr><th>工号</th><th>姓名</th><th>账号</th><th>手机号</th><th>角色</th><th>状态</th><th>操作</th></tr></thead><tbody>' +
    staffs.map((s) => "<tr><td class='mono'>" + esc(s.empNo) + "</td><td>" + esc(s.name) + "</td><td>" + esc(s.account) + "</td><td>" + esc(s.phone) + "</td><td>" + roleCn(s.role) + "</td><td>" +
      (s.status === "active" ? '<span class="chip chip-ok">启用</span>' : '<span class="chip chip-err">禁用</span>') + "</td><td>" +
      '<button class="btn sm st-role" data-id="' + s.id + '">改角色</button> <button class="btn sm st-pwd" data-id="' + s.id + '">重置密码</button> <button class="btn sm st-toggle" data-id="' + s.id + '" data-st="' + s.status + '">' + (s.status === "active" ? "禁用" : "启用") + "</button></td></tr>").join("") +
    "</tbody></table>");
};

/* ============ 客服工作台：微信式双栏（会话列表 + 聊天窗） ============ */
let csWS = null;          // WebSocket 实时通道
let csPollTimer = null;   // HTTP 轮询兜底
let csCurrentUid = 0;     // 当前打开的用户
let csAfter = 0;          // 当前会话已读取到的消息 id

function csStopRealtime() {
  if (csPollTimer) { clearInterval(csPollTimer); csPollTimer = null; }
  if (csWS) { try { csWS.onclose = null; csWS.close(); } catch (e) {} csWS = null; }
}
function csStartRealtime() {
  csStopRealtime();
  const token = svcTkToken();
  if (!token) return; // 未登录不发起 WS，HTTP 轮询兜底也不 needed（会被 adminGuard 拦截）
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  try {
    csWS = new WebSocket(proto + "//" + location.host + "/api/admin/ws/chat?token=" + encodeURIComponent(token));
    csWS.onmessage = (ev) => {
      try {
        const m = JSON.parse(ev.data);
        if (m.type === "message") {
          if (m.userId === csCurrentUid || !csCurrentUid) csPollMessages(true);
          refreshCsList();
        } else if (m.type === "sessions_changed") refreshCsList();
      } catch (e) {}
    };
    csWS.onclose = () => { csWS = null; };
  } catch (e) { csWS = null; }
  // HTTP 轮询兜底：WebSocket 不可用/断线时仍能收到消息
  csPollTimer = setInterval(() => { csPollMessages(false); refreshCsList(); }, 3000);
}

async function refreshCsList() {
  const box = $("#csContactList");
  if (!box) return;
  const data = await admApi("/api/admin/cs/sessions").catch(() => null);
  if (!data) return;
  const sessions = data.sessions || [];
  box.innerHTML = sessions.length ? sessions.map((s) =>
    '<div class="wx-contact ' + (s.userId === csCurrentUid ? "on" : "") + '" data-uid="' + s.userId + '" data-avatar="' + esc(s.avatar || "🙂") + '">' +
    avatarHTML(s.avatar, "wx-avatar") +
    '<div class="wx-contact-main"><div class="row"><b>' + esc(s.userName || ("用户" + s.userId)) + '</b><span class="tiny faint ml-auto">' + esc(s.updatedAt || "") + '</span></div>' +
    '<div class="tiny faint">' + (s.state === "agent" ? "● 接待中" : s.state === "waiting" ? "● 排队中" : "智能客服会话") + '</div></div></div>').join("")
    : '<div class="faint tiny" style="padding:16px">暂无用户会话</div>';
  box.querySelectorAll(".wx-contact").forEach((el) => {
    el.onclick = () => openCsChat(Number(el.dataset.uid), el.querySelector("b").textContent, el.dataset.avatar);
  });
}

async function csPollMessages(force) {
  if (!csCurrentUid) return;
  const data = await admApi("/api/admin/cs/messages?userId=" + csCurrentUid + "&afterId=" + csAfter).catch(() => null);
  if (!data || !data.messages) return;
  if (data.messages.length) {
    const list = $("#wxMsgs");
    if (list) {
      list.innerHTML += data.messages.map((m) => chatBubble(m, "agent")).join("");
      list.scrollTop = list.scrollHeight;
    }
    csAfter = data.messages[data.messages.length - 1].id;
  } else if (force) {
    // 全量刷新场景（首次打开时 csAfter=0 会走这里）
  }
}

function openCsChat(uid, name, avatar) {
  csCurrentUid = uid;
  csAfter = 0;
  const panel = $("#wxChat");
  if (!panel) return;
  panel.innerHTML =
    '<div class="wx-chat-head">' + avatarHTML(avatar, "wx-avatar") + '<b class="ml8">' + esc(name) + '</b><span class="tiny faint ml-auto">用户ID ' + uid + '</span></div>' +
    '<div id="wxMsgs" class="wx-msgs"></div>' +
    '<div class="wx-input-row"><input class="input flex1" id="wxInput" placeholder="发消息…（Enter 发送）"><button class="btn btn-primary" id="wxSend">发送</button></div>';
  const send = async () => {
    const input = $("#wxInput");
    const content = input.value.trim();
    if (!content) return;
    input.value = "";
    await admApi("/api/admin/cs/reply", "POST", { userId: uid, content }).catch((err) => toast(err.message));
    csPollMessages(true);
  };
  $("#wxSend").onclick = send;
  $("#wxInput").onkeydown = (e) => { if (e.key === "Enter") send(); };
  csPollMessages(true);
  refreshCsList();
}

routes["m-cs"] = async function () {
  const g = adminGuard("m-cs");
  if (g !== null) return g;
  const duty = await admApi("/api/admin/cs/duty").catch(() => ({ duty: false }));
  state._after = () => {
    $("#dutyBtn").onclick = () => {
      admApi("/api/admin/cs/duty", "POST", { on: !duty.duty }).then(() => { toast(duty.duty ? "已切换为下班" : "已切换为在岗"); render(); }).catch((err) => toast(err.message));
    };
    refreshCsList();
    csStartRealtime();
    loadTickets(svcTkToken());
  };
  // 离开页面时关闭实时通道
  state._beforeRender = () => csStopRealtime();
  return adminShell("m-cs",
    '<div class="row" style="justify-content:space-between"><div class="h2">客服工作台</div>' +
    '<span class="tiny faint">实时通道：WebSocket + HTTP轮询兜底</span>' +
    '<button class="btn ' + (duty.duty ? "btn-primary" : "") + '" id="dutyBtn">' + (duty.duty ? "模拟下班" : "模拟上班") + '</button></div>' +
    '<div class="wx-shell mt16">' +
    '<div class="wx-sidebar"><div class="wx-sidebar-head">消息</div><div id="csContactList" class="wx-contact-list"></div>' +
    '<div class="wx-sidebar-head mt8">工单</div><div id="ticketList" class="wx-ticket-list"></div></div>' +
    '<div class="wx-chat" id="wxChat"><div class="faint" style="margin:auto;text-align:center">← 点击左侧用户开始接待<br><span class="tiny">实时消息自动刷新，无需手动刷新页面</span></div></div>' +
    '</div>');
};

function svcSessionName(row) { return (row.userName || "用户" + row.userId); }
function svcTkToken() { const info = JSON.parse(localStorage.getItem("locker_admin") || "null"); return info && info.token; }

function openCsPanel(uid, name) {
  const info = JSON.parse(localStorage.getItem("locker_admin") || "null");
  let after = 0;
  const panel = $("#csPanel");
  panel.innerHTML = '<div class="bold14 mb10">接待：' + esc(name) + '</div><div id="csMsgs" class="chat-list" style="max-height:220px"></div>' +
    '<div class="row gap8 mt10"><input class="input flex1" id="csInput" placeholder="回复用户…"><button class="btn btn-primary" id="csSend">回复</button></div>';
  const poll = async () => {
    const data = await admApi("/api/admin/cs/messages?userId=" + uid + "&afterId=" + after).catch(() => null);
    if (data && data.messages && data.messages.length) {
      after = data.messages[data.messages.length - 1].id;
      $("#csMsgs").innerHTML += data.messages.map((m) => chatBubble(m, "agent")).join("");
      $("#csMsgs").scrollTop = $("#csMsgs").scrollHeight;
    }
  };
  poll();
  if (openCsPanel._timer) clearInterval(openCsPanel._timer);
  openCsPanel._timer = setInterval(poll, 3000);
  $("#csSend").onclick = async () => {
    const input = $("#csInput");
    const content = input.value.trim();
    if (!content) return;
    input.value = "";
    await admApi("/api/admin/cs/reply", "POST", { userId: uid, content }).catch((err) => toast(err.message));
    poll();
  };
}

// 兼容历史脏数据：早期误存 URL 编码的中文，显示时解码一次
function decodeTicketText(s) {
  if (!s) return s;
  if (/%[0-9A-Fa-f]{2}/.test(s)) { try { return decodeURIComponent(s); } catch (e) {} }
  return s;
}
async function loadTickets(token) {
  const data = await admApi("/api/admin/tickets?status=pending").catch(() => ({ tickets: [] }));
  const list = data.tickets || [];
  const el = $("#ticketList");
  if (el) el.innerHTML = list.length ? list.map((t) =>
    '<div class="card mb10"><div class="row" style="justify-content:space-between"><b class="mono tiny">' + esc(t.ticketNo) + '</b><span class="chip chip-warn">待处理</span></div>' +
    '<div class="tiny mt5">' + esc(decodeTicketText(t.question)) + '</div><div class="row gap8 mt8"><input class="input flex1 tk-reply" placeholder="处理回复"><button class="btn sm tk-go" data-id="' + t.id + '">结案</button></div></div>').join("")
    : '<div class="faint tiny">暂无待处理工单</div>';
  document.querySelectorAll(".tk-go").forEach((b) => {
    b.onclick = () => {
      const reply = b.parentElement.querySelector(".tk-reply").value.trim();
      admApi("/api/admin/tickets/" + b.dataset.id + "/resolve", "POST", { reply }).then(() => { toast("已结案"); loadTickets(token); }).catch((err) => toast(err.message));
    };
  });
}

/* 启动：无 hash 时按端补齐默认视图（replaceState 不触发额外渲染） */
if (!location.hash) history.replaceState(null, "", "#" + defaultView());
render();
