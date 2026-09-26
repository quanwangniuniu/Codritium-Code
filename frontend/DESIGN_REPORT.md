# AICH Demo — Design Report

构建日期：2026-05-08
范围：v0 demo（KICKOFF.md §7 M0–M5 子集），全本地、全 mock 外部依赖
目标读者：John + Ray

---

## §1 概览

`demo/` 跑通了 SPEC v0.5 §3 的端到端用户路径骨架——首页 → 题目页 → 提交 → 5 维度评分页 → 个人主页——全部本地运行，**所有外部依赖（Supabase / GitHub OAuth / E2B / Anthropic / Trigger.dev / Stripe）已 mock**。理由：默认参数 + 全自主执行的约束下，任何真实集成都需要凭据/账号，user 没有给。

验证结果（dev server + production build 双通过）：

| 路径 | 行为 |
|---|---|
| `GET /` | 200，渲染产品定位 + Daily Challenge 卡 + 5 维度介绍 + 公司专题预览 |
| `GET /login` | 200，mock 登录按钮（一键签为 seed 用户） |
| `GET /problems` | 200，列出 1 道 seed 题（含 difficulty/category/company tag） |
| `GET /problems/:id`（未登录）| 307 → /login |
| `GET /problems/:id`（登录后）| 200，题目描述 + starter files + 提交表单 |
| `POST /api/grade` | 200，运行 mock grader，返回 submission_id |
| `GET /submissions/:id` | 200，5 维度 ScoreBar + 总分圈 + 4 anti-pattern 信号 + 测试结果 |
| `GET /profile` | 200，streak + 历史提交 |
| `POST /api/auth/logout` | 303 → / |

`npx tsc --noEmit`、`npx eslint --max-warnings=0`、`npm run build` 全部 0 error 0 warning。

---

## §2 实际架构 vs SPEC v0.5 §5 计划

SPEC §5 给出的目标栈：Next.js 15 App Router + TypeScript + Tailwind + Shadcn + Supabase Auth/Postgres/Storage + Anthropic SDK + E2B Firecracker + Trigger.dev + Stripe + Resend + Vercel + Sentry。

demo 实施的对齐情况：

| 层 | SPEC v0.5 计划 | demo v0 实际 | 状态 |
|---|---|---|---|
| Frontend framework | Next.js 15 App Router + TS | Next.js 15.5.16 + React 19.1 + TS strict | ✅ 一致 |
| Styling | Tailwind + shadcn/ui | Tailwind v4 + 自写 UI primitives（Button / Card / Badge / Input / Textarea / Select / Label） | 🟡 等价（见 §3.1） |
| Backend API | Server Actions + Route Handlers | Server Actions（login）+ Route Handlers（/api/grade, /api/auth/logout） | ✅ 一致 |
| Auth | Supabase Auth (LinkedIn/Google/GitHub OAuth) | Cookie-based mock auth（`aich_user_id` cookie，固定 seed 用户） | ❌ mock |
| DB | Supabase Postgres | Module-level singleton `Map<string, T>` + seed file | ❌ mock |
| File Storage | Supabase Storage | submission files 存内存 | ❌ mock |
| LLM Grader | Anthropic SDK + G-Eval rubric prompts | 启发式 mock grader（`src/lib/grader.ts`），输出 schema 与 D_geval_rubric_templates §7 100% 对齐 | ❌ mock（schema 对齐） |
| Code Sandbox | E2B Firecracker microVM | 无沙箱；针对 cart 题 hard-coded 测试结果（检查 `round(` / `Decimal` 是否出现在 cart.py） | ❌ mock |
| Async Jobs | Trigger.dev | 同步执行（grading 在请求线程内完成，<10ms） | ❌ mock |
| Email | Resend | — | 🚫 N/A |
| Payment | Stripe | — | 🚫 N/A |
| Deployment | Vercel | 本地 `npm run dev` / `npm run build` | 🚫 N/A |
| Monitoring | Vercel Analytics + Sentry | — | 🚫 N/A |

`demo/.next` 生成的 Route Manifest（`npm run build` 输出）：

```
ƒ /                       Server-rendered on demand
ƒ /login                  
ƒ /problems               
ƒ /problems/[id]          
ƒ /profile                
ƒ /submissions/[id]       
ƒ /api/auth/logout        
ƒ /api/grade              
```

所有路由都是 dynamic SSR（因为读取 cookie + 调用 mock store）。production v0.5 应该多数页面也 dynamic（auth-aware + 实时数据）。

---

## §3 技术决策日志

### 3.1 没用 shadcn/ui CLI

**选择**：自写 7 个 UI primitives（`src/components/ui/*.tsx`），不跑 `npx shadcn add`。
**理由**：shadcn/ui 当前主分支与 Tailwind v4 + React 19 + Next 15.5 的兼容尚有边角问题（@radix-ui 部分组件在 React 19 下需手动调整 ref 类型）；本 demo 只需要 Button/Card/Badge/Input/Textarea/Select/Label 7 个最薄基础原语，自写约 200 行 + 0 依赖（仅 clsx + tailwind-merge）。
**production v0.5 影响**：进入 v0.5 时再决定是否整体切到 shadcn registry（用例可能扩到 Dialog / Sheet / Tooltip / DropdownMenu）。当前自写原语样式与 shadcn token 体系一致，迁移成本低。

### 3.2 Tailwind v4 而非 v3

**选择**：用 `create-next-app@15` 默认拉的 Tailwind v4（PostCSS plugin 形态，`@tailwindcss/postcss`），`globals.css` 用 `@import "tailwindcss";` + `@theme inline` 块。
**理由**：v4 是 Next 15 当前默认；v3 → v4 切换语法差异不大（主要是配置从 `tailwind.config.js` 转移到 CSS 内 `@theme`），对本 demo 无影响。
**production v0.5 影响**：保持 v4，无需切换。

### 3.3 Mock Auth 用 plain cookie 而非 NextAuth/Auth.js

**选择**：`src/lib/auth.ts` 用 Next.js 15 `cookies()` API 直接 set/get `aich_user_id`，登录是 server action，logout 是 route handler。
**理由**：v0.5 的真实选型是 Supabase Auth（per R4）。NextAuth 是另一个候选但 SPEC 已选定 Supabase。引入任何真实 OAuth provider 都会强迫 user 创建 GitHub OAuth App + 提供 secrets——违反"全自主执行"。Cookie + 固定 seed user 是最低代价的占位 + 保持 server-side auth 的 API 形状（`currentUser()` 在 v0.5 切到 Supabase 时只需替换实现，调用点不变）。
**production v0.5 切换**：把 `src/lib/auth.ts` 从读 cookie 改成调用 Supabase `getUser()`。

### 3.4 Mock Data Store 用 module-level globalThis Map

**选择**：`src/lib/store.ts` 用 `globalThis.__aichStore` 全局单例 + `Map<string, T>`，seed 数据在 `src/data/seed.ts`。
**理由**：HMR 模式下 Next dev 会反复重载模块，导致普通 module-level Map 在每次代码改动后被重置。挂在 `globalThis` 上是 Next 文档推荐的 dev 模式 singleton 写法。生产中 store 会被替换为 Supabase 客户端。
**已知 limitation**：服务进程重启时所有非 seed 数据（用户提交）丢失。demo 不持久化是预期。

### 3.5 Grader 启发式而非 LLM

**选择**：`src/lib/grader.ts` 写了一个 deterministic 启发式评分器，输出 schema 与 `D_geval_rubric_templates.md §7` 一致：5 维度（每维 score + reasoning）+ 4 anti-pattern flags（triggered + evidence）+ aggregate total 0–100 + 权重矩阵。
**评分逻辑**：
- **Correctness**：cart 题专用——检测 `round(` 或 `Decimal` 是否出现在 cart.py。出现 → 4.5；改动了但没出现 → 2.0；完全没改 → 1.0。同时返回 4 个 mock test 结果（其中 `test_floating_point_drift` 跟修复挂钩）。
- **AI Collaboration**：扫描 prompt_history 文本，根据 `no, instead | actually | undo`（course corrections）、`why | explain`（asked AI for reasoning）、`diagnose | investigate`（diagnose-first）、长度等加分。
- **Verification**：test pass rate 主导 + 看 prompt 是否提到 test/verify/check。
- **Decomposition**（仅 Hard 启用）：扫 `plan / step / first then / clarify`。
- **Communication**（仅 Hard 启用）：扫 `because / so that / trade-off`。
- **Anti-patterns**：Hands-Off = `num_prompts ≤ 1` + 有 prompt 文本；Feature Marathon = `num_prompts > 12`；AI Showcase = 关键字 `showcase / advanced technique`；Not Thinking = 关键字 `just fix it / rewrite this / do whatever`。
- **Aggregate**：完全按 rubric §7 的权重矩阵 + anti-pattern penalty。

**实测**：好提交（有 round() + 4 prompts + 提到 docstring contract + 提到 IEEE 754）→ 总分 83；坏提交（没改 + 1 prompt 内容是 "just fix it"）→ 总分 34，触发 hands_off + not_thinking。区分度足够展示 UI。

**production v0.5 切换**：替换 `gradeSubmission()` 为对 5 个维度的 5 个并行 Anthropic API call + Anti-pattern detection 的第 6 个 call，输出 schema 不变。

### 3.6 Markdown 渲染：自写最小子集而非 react-markdown

**选择**：`src/components/markdown.tsx` 自写 ~50 行，支持 h1/h2/h3 + 有序无序列表 + 段落 + 行内 code/strong/em。
**理由**：题目描述都在我们控制，不需要完整 CommonMark；引入 react-markdown + rehype 链路 ≈ 增加 ~50KB bundle，对 demo 不值。Markdown 内容做了 HTML escape 防 XSS。
**production v0.5 影响**：题目描述最终走 `react-markdown + remark-gfm`，本 demo 提供的 typography 样式 token 直接套用。

### 3.7 题目内容选型：cart_total floating-point 漂移

**选择**：v0 demo 的唯一题就是 cart_total floating-point 精度 bug（Easy / debugging）。
**理由**：(a) Microcosm 派——真实工程问题（购物车精度是真实生产 bug 类）；(b) 修复路径明确（`round()` 或 `Decimal`）便于 mock grader 给出确定性评分；(c) 体量小（~10 行 starter）便于在表单 textarea 内编辑；(d) docstring contract 与 implementation 不一致 = 经典 debugging 题型。Ray 私有题源进入 v0.5 后这道 demo 题会被 production 题库替换。
**Hard 模糊化前缀范本**：本 demo 没写。SPEC §3.1 + KICKOFF §6 P1 列在 Ray-pending；样例可在 Ray 同步后追加。

---

## §4 已知 limitations（demo → 真实差距）

按用户路径列出每一段哪里是 mock：

1. **登录**
   - mock：点 "Continue as demo user" → cookie 直接设为 `demo-user-001`
   - 真实：Supabase Auth GitHub OAuth → JWT → cookie

2. **题库**
   - mock：1 道题硬编码在 `src/data/seed.ts`
   - 真实：Supabase `problems` 表（13 表 schema 见 `D_supabase_schema.md` §1.3）+ Storage bucket 存 starter_files（如题超过 5KB）

3. **题目页**
   - mock：starter_files / answer_test_file / hint 都从 in-memory Map 读
   - 真实：从 Supabase row 读 + RLS（Pro-only 题需订阅）

4. **提交**
   - mock：表单 POST → `/api/grade` 同步执行 grader → 返回 submission_id
   - 真实：表单 POST → 创建 Supabase `submissions` row（status=pending）→ 触发 Trigger.dev job → job 内调 E2B 跑 pytest + 调 Anthropic SDK 5 次（5 维度独立 call）+ 第 6 次 call 跑 anti-pattern detection → 写回 `submission_scores` + `submission_anti_pattern_flags` → 前端 polling status → 跳转

5. **评分**
   - mock：`src/lib/grader.ts` 启发式逻辑（关键字 + 数值规则）
   - 真实：每维度独立 Claude Sonnet 4.6 call，rubric prompt 已有模板（`D_geval_rubric_templates.md` §1–§5），输出 schema 已对齐

6. **代码沙箱**
   - mock：cart 题专用 hard-coded test results；非 cart 题 fallback 到 generic pass/fail
   - 真实：E2B Firecracker microVM + 真实 pytest + ruff/mypy/bandit static analysis

7. **Streak 计算**
   - mock：`SEED_USER.streak_current = 7`（硬编码）；提交后不会更新
   - 真实：每日提交后 cron 计算（per `D_supabase_schema.md` §1.1 streak fields），streak 算法考虑评分质量门槛

8. **Daily Challenge**
   - mock：今天的题 = `prob-cart-float-001`（每天都同一道）
   - 真实：cron 每天 00:00 UTC（user timezone-aware）插入 `daily_challenges` row + 推送

9. **Pro tier**
   - mock：seed user `is_pro = false`；UI 上有 Pro badge 但没有 paywall
   - 真实：Stripe webhook → `subscriptions` 表 → `user_profiles.is_pro = true` + RLS 控制 Pro-only 题访问

10. **Workflow Replay / Explanation Video / Study Plan UI / 公司专题 UI / Beta 邀请**
    - 全部不在 v0 demo 范围（KICKOFF §7 明确不入 v0）

---

## §5 v0.5 实施时的接入顺序（demo → production）

按"换组件 + 跑 migration + 接 API"维度组织，每步独立可上线：

1. **Supabase 项目 init + Migration**
   - 跑 `D_supabase_schema.md` §4 列出的 13 表 + 3 view 的 migration
   - 创建 RLS 策略
   - Seed `companies` (canva / atlassian) + 第一批题
   - 替换：`src/lib/store.ts` 的所有 getter 改成 Supabase Server Client 调用，类型保持不变

2. **Supabase Auth 接入**
   - 配置 GitHub OAuth provider（需 user 提供 OAuth App credentials）
   - 替换：`src/lib/auth.ts` 的 `currentUser` / `loginAs` / `logout` 改成 `supabase.auth.*`
   - 上线 row：`user_profiles` 在第一次登录时自动 upsert（trigger function on `auth.users` insert）

3. **E2B 沙箱接入**
   - 注册 E2B 账号 + 拿 API key
   - 在 `/api/grade` 内创建 microVM → 写入 submitted_files + answer_test_file → 跑 `pytest --json-report` → 解析 → 关 microVM
   - 替换：`src/lib/grader.ts` 的 `gradeCorrectness` 内 cart 专用逻辑去掉，改读真实 pytest 结果

4. **Anthropic LLM Grader**
   - Anthropic API key + node SDK
   - 5 个并行 call（Correctness 仅 Refactor/FB/Security 类调；Decomposition / Communication 仅 Hard 调）
   - prompt 模板从 `D_geval_rubric_templates.md` §1–§5 复制即可
   - 替换：`gradeAiCollaboration` / `gradeVerification` / `gradeDecomposition` / `gradeCommunication` 改为 LLM call；保留启发式 grader 作为 fallback（API down 时降级）

5. **Trigger.dev 异步队列**
   - 把 `/api/grade` 的 grading 部分挪到 Trigger.dev job
   - 路由立即返回 submission_id（status=pending）
   - 前端 SSE 或 polling 追 status

6. **Stripe + Pro tier paywall**
   - Stripe Pro plan webhook → `subscriptions` 表
   - RLS 在 `problems` 表已经写了，加价后自动生效

7. **Daily Challenge cron**
   - Vercel cron 或 Trigger.dev cron 每日触发
   - 写 `daily_challenges` row

每步都不破坏已有功能；迭代可拆 PR。

---

## §6 文件清单

```
demo/
├── package.json                Next 15.5.16 + React 19 + Tailwind v4 + lucide-react + clsx + tailwind-merge
├── next.config.ts              空 config
├── tsconfig.json               default Next config，paths "@/*" → "./src/*"
├── eslint.config.mjs           default flat config
├── postcss.config.mjs          @tailwindcss/postcss
├── DESIGN_REPORT.md            本文件
└── src/
    ├── app/
    │   ├── layout.tsx          全局 Nav + Geist font
    │   ├── page.tsx            首页（Daily Challenge + 5 维度 + 公司预览）
    │   ├── globals.css         Tailwind v4 + design tokens
    │   ├── login/page.tsx      Mock login（server action）
    │   ├── problems/
    │   │   ├── page.tsx        题目列表
    │   │   └── [id]/page.tsx   题目详情 + 提交表单
    │   ├── submissions/[id]/page.tsx   5 维度评分页
    │   ├── profile/page.tsx    Streak + 历史
    │   └── api/
    │       ├── auth/logout/route.ts
    │       └── grade/route.ts  POST → run grader → save → return id
    ├── lib/
    │   ├── types.ts            User / Problem / Submission / ScoreBreakdown / AntiPatternBundle 等
    │   ├── store.ts            globalThis-pinned in-memory store
    │   ├── auth.ts             cookie-based mock auth
    │   ├── grader.ts           启发式 5 维度 grader + 4 anti-pattern detection
    │   └── utils.ts            cn() helper
    ├── data/
    │   └── seed.ts             SEED_USER + SEED_COMPANIES + SEED_PROBLEMS + SEED_DAILY + SEED_STUDY_PLANS
    └── components/
        ├── ui/
        │   ├── button.tsx
        │   ├── card.tsx
        │   ├── badge.tsx
        │   ├── input.tsx
        │   ├── textarea.tsx
        │   ├── select.tsx
        │   └── label.tsx
        ├── nav.tsx                顶部导航（auth-aware + StreakFlame）
        ├── streak-flame.tsx       Lucide Flame + count
        ├── score-bar.tsx          5 维度水平条 + reasoning
        ├── total-score-circle.tsx 总分圈（SVG ring）
        ├── anti-pattern-chip.tsx  4 信号 chip
        ├── code-block.tsx         只读代码块
        ├── markdown.tsx           最小 markdown 渲染器
        ├── daily-challenge-card.tsx
        └── submission-form.tsx    Client component（"use client"）
```

总：~30 文件，约 1500 行 TS/TSX。

---

## §7 跑通命令

```bash
cd /Users/johns3248/project/AICH/demo
npm run dev      # 已验证：localhost:3000 (or :3001 if 3000 busy)
npm run build    # 已验证：production build 0 error
npx tsc --noEmit # 已验证：0 type error
npx eslint src   # 已验证：0 warning
```

dev server 跑起来后试一下：
1. http://localhost:3001 — 首页 + Daily Challenge
2. 点 "Sign in" → 一键登录
3. 点 "Browse problems" → 进 cart 题
4. 编辑 cart.py（在 textarea 里把 `return total` 改成 `return round(total, 2)`）
5. 在 prompt history 写一段 narration
6. 点 "Submit for grading" → 跳到 /submissions/:id 看 5 维度评分

---

## §8 下次启动 checklist

新 session 接手 demo（或推进到 v0.5）时：

### 当前 demo 已完成
- ✅ 项目骨架 + 类型 + mock store + mock auth + UI primitives
- ✅ 端到端流程：home → login → problem → submit → score → profile
- ✅ Mock grader 输出 schema 与 D_geval_rubric_templates §7 100% 对齐
- ✅ 类型检查 + lint + production build 全通过

### v0.5 第一波（如启动）需要 user 提供的真实凭据
- Supabase 项目 URL + anon/service key
- GitHub OAuth App client_id + client_secret（callback URL 见 Supabase 文档）
- E2B API key
- Anthropic API key
- Trigger.dev project token（如启用异步）
- Stripe API key + webhook secret（如启用 Pro tier）
- Resend API key（如启用邮件）

未拿到任一凭据前，相应模块保持 mock 状态。

### 阻塞 v0.5 启动的非技术项（per KICKOFF §6 P0）
1. Ray 同步反馈（R2 §5 brief / R3 §4 / SPEC §11 7 项 / R4 技术栈认可 / D_geval pending §9 / D_supabase pending §5）
2. 题库 v0.5 子集 12 道（Ray 选）
3. Hard 题 Canva 模糊化前缀 9 个范本（Ray 写）
4. Pro 定价
5. 域名

### Demo 自身延伸（可选）
- 多题 seed（debugging / refactoring / feature_build / security 各 1 道）展示 difficulty 分布
- 模拟 Hard 题（含 canva_fuzzification_prefix_md 字段渲染）
- Workflow Replay 占位 UI（v0.6 真实功能前的形状探索）
- 用 SQLite + better-sqlite3 替换 in-memory Map（持久化但仍无外部依赖）

---

## §9 与 KICKOFF §7 验收标准的对照

| §7 验收点 | demo 状态 |
|---|---|
| 首页（含产品定位 + Daily Challenge 入口）| ✅ |
| 1 道题页面（题目描述 + upload 提交）| ✅ |
| 提交后跳转评分结果页（5 维度 + 4 anti-pattern flags + 总分）| ✅ |
| User profile 页（streak counter + 历史提交）| ✅ |
| Login / Logout（GitHub OAuth）| 🟡 mock 实现，OAuth 部分按计划替换 |

---

## §10 Layout v2（基于 R5 竞品调研）

R5 调研 15 个 coding-practice 平台后（见 `research/R5/S_R5_summary.md`），demo 在不引入新外部依赖的前提下完成了 4 项布局升级。变更范围控制在 §4 v2 推荐的最小可行集，跳过了"`/problems` filter+table"和"thin icon left rail"——前者条件 §4 自身写明 seed > 6 题再启用，后者依赖前者，当前 1 道题不达条件。

### 新增组件
| 组件 | 路径 | 用途 |
|---|---|---|
| `Breadcrumbs` | `src/components/breadcrumbs.tsx` | 面包屑导航，问题详情页与提交页共用 |
| `CalendarHeatmap` | `src/components/calendar-heatmap.tsx` | 12 周 × 7 天分数加权 heatmap，月份标签 + 强度色阶图例 |
| `CategoryMastery` | `src/components/category-mastery.tsx` | 4 类别 × 平均分进度环（Debugging / Refactoring / Feature build / Security） |

### 页面级变更

**`/profile`**：原 3 张 stat cards 替换为单卡片 `Activity` 区——左为 12 周 heatmap、右为 Current/Longest/Total 三个数字 + Category mastery 环。Submission history 上方新增类别 filter chips（query string `?category=`）。

**`/problems/[id]`**：从单列 max-w-5xl 改为 max-w-7xl 双栏。左栏 sticky 描述区带 tabs（Brief / Hint / Anti-patterns），右栏为 Starter 文件 + SubmissionForm。顶部新增 Breadcrumbs `Problems > {category} > {title}`。Anti-patterns tab 内联 4 个反模式的简介文字，作为 grader 信号的非程序化教学位。

**`/submissions/[id]`**：新增 220px 左侧 Iterations rail——列出该用户在该题的所有提交，按时间倒序，每条显示 #序号 + 总分 + 日期 + 与上一次提交的 score delta（绿/红/灰）。Active 提交用 `bg-accent-soft` + `border-accent` 高亮。底部 `Resubmit` 按钮直链回 `/problems/[id]`。顶部 Breadcrumbs `Problems > {title} > Submission #N`。

**未变更项**：
- `/`（home）保留现有 hero + 5-dim 介绍 + 公司 AI policy 三段，§2 takeaway 表明这种 marketing 密度对 logged-out 公开页是行业共识
- `/submissions/[id]` 的 5 维度分数面板 + 4 反模式 chips 保持原 3-col 布局，§4 已说明这是 AICH 的差异化所在，不需要趋同

### Store 新增 helper
- `listUserProblemSubmissions(userId, problemId)` —— 支持 iteration rail 查询

### 自验证（bb-browser）
浏览器截图见 `demo/screenshots/v2/`：
- `01-login.png` `02-home.png` `03-profile-empty.png` `04-problem-detail.png` `05-submission.png` `06-submission-iter2.png`

`05`/`06` 验证 iteration rail 在多次提交后正确显示 #1 (83) → #2 (34) 倒序排布并高亮当前条目。


5 项中 4 项原生达标，第 5 项 mock + 替换路径清晰（§5 step 2）。
