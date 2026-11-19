# Smart-Locker 智能寄存柜

Go + MySQL + vanilla SPA 三端模拟实现（支付/短信/地图均为模拟，验证码固定 `1234`）。

## 启动

前置：本地 MySQL `127.0.0.1:3306`（root/root），库 `smart_locker`（迁移启动时自动执行）。

```bash
go run ./cmd/server
```

## 端口（三端独立）

| 端口 | 端 | 入口 | 说明 |
| --- | --- | --- | --- |
| `8081` | 用户小程序端 | `#u-home` | 手机视图（430px 居中），登录/网点/存件/取件/订单/卡券/客服/我的 |
| `8082` | 柜机触屏端 | `#k-standby` | 横屏深色（10.1 寸），待机/存件/取件 |
| `8083` | 管理后台 | `#m-login` | 全屏数据视图，11 个模块 |
| `8090` | 兼容总入口 | `#u-home` | 三端均可访问（e2e / 冒烟 / five_scenarios 使用） |

端口可用环境变量覆盖：`LOCKER_USER_ADDR`、`LOCKER_KIOSK_ADDR`、`LOCKER_ADMIN_ADDR`、`LOCKER_ADDR`。
DSN 用 `LOCKER_DSN` 覆盖；`LOCKER_DSN=mem` 可无数据库运行（数据不持久化）。

## 演示账号

- 用户端：任意手机号 + 验证码 `1234`
- 后台：`super`（超管）/ `site_rm`（网点管理员）/ `svc`（客服），密码均 `000000`

## 测试与验收

```bash
# 覆盖率门禁（整体 ≥85%）
bash scripts/check_coverage.sh 85

# 前端全量巡检（三端 28 路由 + 登录控件 + JS 错误）
node scripts/diag_all.js

# 三端口验证
node scripts/diag_ports.js

# e2e 全流程 + 五大场景
node scripts/e2e_check.js
powershell -ExecutionPolicy Bypass -File scripts/five_scenarios.ps1
```

报告：`docs/测试报告.md`、`docs/验收报告.md`。
约束与假设：`AGENTS.md`、`docs/假设清单.md`。
