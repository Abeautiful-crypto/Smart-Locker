# AI 使用说明与自编 Skill 总结（#605）

## 1. AI 开发过程说明

本项目按题目共同要求的七阶段完成：需求分析 → 原型设计 → 任务规划 → Agent 开发 → Agent 测试 → Agent 验收 → 质量门禁与交付。三角色（开发/测试/验收）使用相互独立的会话上下文（AGENTS.md 第 1 节），测试与验收 Agent 均基于原始需求文档与代码独立取证，不采信开发声明。

- 需求分析：`docs/智能寄存柜-需求分析.md`（依据原始 PRD 与交互原型 HTML）
- 原型设计：原型 CSS 原样提取复刻（web/proto.css），SPA 壳 + hash 路由复刻三端交互
- 任务规划：`docs/任务清单.md`（编号 #100–#605，含修订登记）
- 开发：Go 后端（cmd/internal/migrations）+ vanilla SPA（web/）
- 测试：`docs/测试报告.md` + `scripts/five_scenarios.ps1`（五大场景 HTTP 独立验证）
- 验收：`docs/验收报告.md`（逐项结论+证据位置）
- 质量门禁：`.github/workflows/ci.yml`（gofmt + vet + test -race + 覆盖率 ≥85% + 前端语法，MySQL 8 服务容器同口径）

## 2. 自编 Skill 总结（#501/#502 → 提交拆分 #105）

### 2.1 locker-business-rules（业务规则专项）

- 位置：`.codebuddy/skills/locker-business-rules/SKILL.md`
- 用途：开发/修改存取件、支付、结算、优惠券、柜格分配、上报异常、退款代码时，逐条核对题面 8 条计费硬约束、状态机、防超卖、权限边界。
- 实际调用记录：本会话以该 Skill 核对计费口径 8 条，结论"实现（internal/biz/billing.go）/测试（billing_test.go）/DB 默认值（migrations/001_schema.sql）三处一致"；并据此定位并修复时区缺陷 BUG-#402。
- 触发方式：修改上述模块代码，或需要核对计费口径/封顶/免费时长/押金/券结算时。

### 2.2 locker-acceptance-check（验收检查）

- 位置：`.codebuddy/skills/locker-acceptance-check/SKILL.md`
- 用途：交付前验收——环境冒烟、质量门禁独立复跑、五大演示场景、硬错误核查（超卖/重复收款/越权/退款口径）、输出验收报告模板。
- 实际调用记录：按该 Skill 步骤完成 `docs/验收报告.md`（逐项结论+证据）。
- 与业务专项的区别：验收判"能不能交付"，业务专项判"规则实现对不对"。

## 3. AI 工具与模型

- 编码助手：CodeBuddy（Claude 系列模型），MCP 工具：GitHub（建 PR/查状态）、文件读写、搜索、终端。
- 浏览器自动化：Playwright（原型爬取与 e2e）。
- 全部提交经学生确认后由 AI 执行；AI 生成代码已按 AGENTS.md 第 4 节约束接受检查与复测。
