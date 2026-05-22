# WikiMind UI 原型

> 基于三方案审查结论的视觉化原型。两版并存：**v1** 是首次结构验证，**v2** 是 Vercel / Geist 风视觉精修。

---

## 文件

```
prototypes/
├── README.md              ← 你正在读
├── wikimind-ui-v1.html    ← v1 首版（2646 行，零依赖）
└── wikimind-ui-v2.html    ← v2 精修（Geist 字体 + Lucide SVG + Vercel 配色）
```

---

## 怎么看

**推荐**：直接双击 `wikimind-ui-v2.html`。

```bash
open prototypes/wikimind-ui-v2.html        # macOS
xdg-open prototypes/wikimind-ui-v2.html    # Linux
start prototypes/wikimind-ui-v2.html       # Windows
```

**Side-by-side 对比**（看精修差异）：

```bash
open prototypes/wikimind-ui-v1.html prototypes/wikimind-ui-v2.html
```

**建议浏览器窗口宽度 ≥ 1280px**（桌面优先，未做响应式）。
**v2 需联网首次加载 Geist 字体**（Google Fonts，约 50KB），离线 fallback 到 SF Pro 也能正常显示。

---

## v2 做了什么（v1 → v2 的 14 项 Δ）

用户反馈 v1 "**布局没问题，但不够精致**"，拆解为：质感不够 + 留白不够 + 粗糙。对标 **Vercel / Geist 风**。

| # | 维度 | v1 | v2 |
|---|---|---|---|
| A | **字体** | system-ui | **Geist Sans** + **Geist Mono**（Google Fonts），数字全 tabular-nums，开 ss01/cv11 |
| B | **灰阶** | stone（偏暖） | **zinc**（偏冷、更精致） |
| C | **Accent** | 蓝 + 紫双 accent | 单色 **`#0070f3`**（Vercel 蓝）；类型语义色（claim 紫/entity 蓝/concept 绿）只用文字 + 细边 |
| D | **留白** | scene padding 24/32 | **40/56**；卡片 gap 12 → 20；padding 16 → 24 |
| E | **Icon** | emoji + Unicode 符号混用 | 全部替换为 **Lucide SVG**（1.5 stroke / 24 viewBox），用 `<symbol>` + `<use>` 复用 |
| F | **Primary 按钮** | 蓝色 | **纯黑 `#000` + 白字**（Vercel 标志按钮）；dark 反转白 + 黑字 |
| G | **Card** | 多层 box-shadow | 去掉阴影，**1px 细 border + bg 对比** 区分层次 |
| H | **Metric 数字** | 26px / sans / weight 600 | **36px / Geist Mono / weight 500**，letter-spacing -0.04em |
| I | **Chart** | 实色 area + grid line | **Gradient area**（0.18 → 0）、去 grid line、加 peak 标注圆环 |
| J | **Badge** | 实色 bg | **透明 bg + dot + 文字 + 细边**（Vercel 状态指示风格） |
| K | **Diff** | 鲜艳 add/del bg | **极淡色块**（add `#f6fef9` / del `#fdf6f6`），dark 下用 8% alpha |
| L | **Sidebar active** | 实色块 | **左侧 2px 黑色 stripe + 弱 bg**（Linear 风强调） |
| M | **微动效** | 几乎无 | 全局 100-150ms transition；scene 切换 200ms fadein；metric 加载 fadein |
| N | **Dark mode** | bg `#0a0a0a` + border `#27272a` | bg 保持 + border 收弱到 `#1c1c20`，secondary text 收到 `#8b8b91`，accent 用 `#3b82f6` |

**细节扫尾**：滚动条更细更圆、focus ring 2px 黑色 + 2px offset、kbd 改细 1px border + radius 4、health-ring 用 SVG stroke 画 conic 而非 conic-gradient（Safari 渲染一致）。

---

## 4 个场景导航（两版相同）

通过左侧 sidebar 切换：

| Scene | 入口 | 内容 |
|---|---|---|
| **Dashboard** | `Dashboard` | 4 张 metric cards + 14 天活动 timeline（SVG）+ Vault Health 评分 + 3 个行动 tips + 最近 7 条 activity feed |
| **Review Queue** | `Review Queue (12)` | filter bar（agent / type / priority）+ 3 个 bundle（首个展开含 3 张 propose card）+ 右侧 Bundle Inspector |
| **Wiki** | `Wiki (347)` | 左侧 file tree（双链颜色区分）+ 中间 claim 详情页（frontmatter / sources / DRIFT 警告 / history）+ 右侧 Backlinks / Related / Actions / Provenance |
| **Ingest** | `Ingest (3)` | 5 阶段 pipeline + 文件列表（含 OCR 失败、音频转写）+ 实时抽取 + Worker Pool（4 agent） |

灰显项（Claims / Entities / Concepts / Sources / Agents / Lint Rules / Dream Cycle / Settings）标注为 v0.2 占位。

---

## 关键交互（v2 已实现）

- ✅ 左侧 sidebar 切换 4 个场景（scene 切换有 200ms fade）
- ✅ Dark mode toggle（icon 跟着切换 moon / sun）
- ✅ Review：点击 propose card 高亮选中
- ✅ Wiki：点击 file tree item 高亮（紫色为 claim 类型）
- ✅ Ingest：点击文件行高亮
- ✅ Wiki frontmatter 折叠/展开（chevron 旋转）
- ✅ 全局 focus ring（Tab 键导航无障碍）
- ✅ 各元素 hover transition（100-150ms）

---

## v2 关键设计决定（什么被刻意"精致化"）

1. **字体是最大杠杆** — Geist Sans 让所有文本立刻"高了一档"；Geist Mono 让数字、ID、hash、git sha 全有秩序感。
2. **纯黑 primary 按钮** — Vercel 标志手势，传达"这是 dev tool 不是消费品"。
3. **去阴影靠 border** — Vercel 风的视觉骨架，看起来"更平也更精致"。多阴影是 Material 风格，是反 Vercel 的。
4. **单色 accent + 语义色作弱化** — claim/entity/concept 仍区分（紫/蓝/绿），但只用文字色 + 细边，不让它们抢主色 `#0070f3` 的戏。
5. **Sidebar active 用 2px stripe** — 比 v1 的大色块更克制，Linear 经典手势。
6. **数字用 mono + tabular** — 所有 metric value、conf、ID、sha 全部对齐齐整，"严肃产品"的标志。
7. **图表 area 用 gradient** — 比 v1 的实色填充更现代；peak 加圆环高亮是"被精修过"的细节。
8. **Badge 全部去 bg** — 只留 dot + 文字 + 细边，眼睛立刻轻了一档。
9. **Dark mode 不用纯黑** — `#0a0a0a` 而非 `#000`，border 收到 `#1c1c20`，让卡片"漂浮"而不是"贴墙"。
10. **微动效全局统一** — 100ms color / 150ms bg / 200ms transform，所有交互一致的"丝滑感"。

---

## 设计取舍（v2 vs v1）

| 项 | v1 选择 | v2 选择 | 为什么 |
|---|---|---|---|
| 字体 | system-ui 零依赖 | Geist via Google Fonts | "精致"最大杠杆是字体；用户允许 CDN |
| 配色 | 双 accent（蓝 + 紫） | 单 accent（Vercel 蓝） | 极简派的核心是"少一个" |
| 信息密度 | 高，cards 紧贴 | 同样高，但 padding 和 gap 大幅放大 | 留白本身就是"精致"信号 |
| 按钮 primary | 蓝 | 纯黑 | Vercel 标志风格 |
| Box shadow | 多层 sm/md/lg | 全去掉 | 1px border 已够，阴影是"装饰" |
| Icon | emoji + Unicode | Lucide SVG | 视觉一致性的最大单点提升 |

---

## 已知简化（**不在原型范围**）

| 维度 | 简化 | 真实产品需要 |
|---|---|---|
| 交互逻辑 | Accept / Reject 等按钮无后端 | 真正发请求到 daemon |
| 数据 | 全部硬编码 | 从 daemon / SQLite 拉 |
| 路由 | 单页 JS 切换 | 真正的 SPA 路由 |
| 响应式 | 桌面优先 ≥ 1280px | 至少 1024px 兼容；移动端 v1.5 |
| 搜索 | 顶部 search box 不可用 | 全文 + 元数据 search |
| 多 vault | vault 切换器假的 | 真正切换 |
| Diff 渲染 | 折叠按钮点击只显示提示 | 真实 word-level diff |
| 通知 | 顶部 🔔 不可用 | 真实通知中心 |
| 离线字体 | Geist 必须联网首次加载 | 字体打包进 build 产物 |

---

## 看完 v2 后我希望你能给的反馈

1. **够精致了吗？** ——如果"够"，可以回去把 spec 整合做完；如果"还差点意思"，告诉我**具体哪块**：是字体、配色、留白、icon、还是某个组件？
2. **场景配色 / 信息密度还需要调？** ——任何场景哪里看起来还粗？
3. **Dark mode 的氛围对吗？** ——这是 v2 重点精修的部分。
4. **需要做 React 交互版吗？** ——让 Accept/Reject 真的有反馈、bundle 真能展开折叠？
5. **是否回去把 spec 整合做完？** ——原型方向定了之后。

---

## 历史

- **v1** (2026-05-21)：首版原型，4 个场景，light/dark 双主题，零依赖，结构验证。
- **v2** (2026-05-21)：Vercel / Geist 风精修，14 项 Δ，引入 Geist 字体 + Lucide SVG icon，纯黑 primary 按钮，全局微动效。
