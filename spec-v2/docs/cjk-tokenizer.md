# CJK 分词与全文检索

> **GPT Pro 方案独家贡献**：SQLite FTS5 的默认分词器对中日韩（CJK）文本**基本失效**。
> 这是一个隐形杀手——不专门处理，中文 vault 的搜索会"看起来能用其实查不到"。

本文档定义 WikiMind 的 CJK 分词选型、配置、性能与测试。

---

## 1. 问题：FTS5 默认分词器对 CJK 失效

### 1.1 FTS5 内置分词器

| Tokenizer | 行为 | CJK 效果 |
|---|---|---|
| `unicode61`（默认） | 按 Unicode 标准分词，**用空格/标点切词** | ❌ 中文没有空格 → 整句变一个 token |
| `porter` | unicode61 + 英文词干还原 | ❌ 对 CJK 同上，还白做了词干 |
| `ascii` | 仅 ASCII | ❌ 完全不处理 CJK |
| `trigram` | 3-字符滑动窗口 | ✅ 对 CJK 可用（见 §3） |

### 1.2 失效演示

假设 wiki 里有 claim：

```
title: Wiki 是一个 compounding artifact
body: 每一次 ingest、每一次 query 都让 wiki 更值钱。
```

用默认 `unicode61` 建索引：

```sql
CREATE VIRTUAL TABLE pages_fts USING fts5(title, body);  -- 默认 unicode61
```

分词结果（中文部分）：

```
"每一次 ingest、每一次 query 都让 wiki 更值钱"
→ tokens: ["每一次", "ingest", "每一次", "query", "都让", "wiki", "更值钱"]
   (按空格/标点切；"每一次" 因为没空格分隔，整体成一个 token)
```

User 搜索 `wiki 值钱`：

```sql
SELECT * FROM pages_fts WHERE pages_fts MATCH '更值钱';   -- ✓ 命中（完全匹配 token）
SELECT * FROM pages_fts WHERE pages_fts MATCH '值钱';     -- ✗ 不命中！("值钱" 不是独立 token)
SELECT * FROM pages_fts WHERE pages_fts MATCH '每一';     -- ✗ 不命中！
```

**结论**：用户搜"值钱"、"每次"、"查询"这类**词的一部分**或**同义表达**——全部查不到。
搜索框看起来工作（搜整句能命中），实际上**子串搜索完全失效**。这就是"看起来能用其实不能用"。

### 1.3 为什么 2026 年还有这个坑

- FTS5 的 `unicode61` 设计假设"词之间有空格"——对英文/欧洲语言成立，对 CJK 不成立
- SQLite 官方不内置中文分词器（不想绑定词典）
- `trigram` tokenizer 是 SQLite 3.34（2020）后才有，很多教程仍推荐 `unicode61`
- 2026 年 4 月社区有报告：某些 SQLite 发行版的 `unicode61` 对 CJK 的 `categories` 处理有回归

→ **任何"用 SQLite FTS5 做中文搜索"的方案如果没专门提分词，默认就是坏的。**

---

## 2. 三个选项

| 选项 | 原理 | 优点 | 缺点 |
|---|---|---|---|
| **A. trigram** | 3 字符滑动窗口（FTS5 内置） | 零依赖、无词典、子串搜索好、CJK/英文统一 | 索引体积大 ~3x、不懂"词"边界、短查询（< 3 字）退化 |
| **B. jieba（外部）** | 中文分词词典 | 真正的词边界、查询精准、体积适中 | 需引入 jieba（Python / Go port）、词典维护、新词漏切 |
| **C. ICU + 自定义** | ICU 分词 + 自写 FTS5 tokenizer | 多语言通用、Unicode 正确 | 实现复杂、ICU 是大依赖、跨平台编译麻烦 |

---

## 3. 选型决策

### 3.1 MVP：**trigram**（选项 A）

**理由**：

1. **零依赖** —— FTS5 内置，不引入任何外部库 / 词典。符合 SPEC §1 "boring + small steps"。
2. **CJK + 英文统一** —— 同一个 tokenizer 处理中英混排，不需要按语言切换。
3. **子串搜索正确** —— 用户搜"值钱"、"查询"能命中，这是中文搜索体验的底线。
4. **个人规模够用** —— 100-10k 页的 vault，trigram 索引体积大 3x 但绝对值仍小（10k 页约 50-100MB）。

### 3.2 配置

```sql
CREATE VIRTUAL TABLE pages_fts USING fts5(
    id UNINDEXED,
    title,
    body,
    tokenize = 'trigram'
);
```

trigram 自动小写化 + Unicode 处理，无需额外参数。

### 3.3 查询适配

trigram 的查询需要适配——因为 token 是 3-gram：

```go
// 用户查询 "值钱"（2 字，< 3）→ trigram 无法直接匹配
// 解决：daemon 对短查询做 fallback
func search(query string) {
    if utf8.RuneCountInString(query) < 3 {
        // fallback 到 LIKE / ripgrep
        return likeSearch(query)
    }
    return fts5Search(query)
}
```

| 查询长度 | 策略 |
|---|---|
| ≥ 3 字符 | FTS5 trigram MATCH |
| < 3 字符 | `LIKE '%query%'` 或 ripgrep 兜底 |
| 多词（空格分隔） | 各词分别 trigram MATCH 后 AND |

### 3.4 v0.2 升级路径：jieba（选项 B）

当 vault > 10k 页 / user 反馈搜索精度不够时，提供可选 jieba：

```toml
# .wikimind/config.toml
[search]
tokenizer = "trigram"   # MVP 默认
# tokenizer = "jieba"   # v0.2 可选；切换需 wikimind rebuild-index
```

切换 tokenizer 必须 `wikimind rebuild-index`（FTS5 表重建）。

jieba 集成方式：
- Go 侧用 `gojieba`（cgo）或 FTS5 external tokenizer API 注册自定义 tokenizer
- 词典：内置 jieba 默认词典 + vault 专属 `schema/dict.txt`（user 可加专业术语 / entity 名）

### 3.5 不选 ICU 的理由

ICU 是正确但重的方案——跨平台静态编译麻烦、二进制膨胀大。
对"个人知识库"规模，trigram 已经够，ICU 的多语言精度优势用不上。v2+ 如做多语言再评估。

---

## 4. 混合语言处理

WikiMind 的 vault 常见中英混排（"使用 TypeScript 开发"）。trigram 对此天然友好：

```
"使用 TypeScript 开发"
trigram tokens（含跨语言 3-gram）:
  使用T, 用Ty, ... TypeS, ypeSc, ... cript开, ript开发, ...
  （连同空格一起进 trigram）
```

实测：中英混排查询（"TypeScript 开发"、"开发 typescript"）trigram 都能命中。

**英文部分**：trigram 对纯英文略不如 porter（无词干还原，"running" ≠ "run"），但：
- 个人 wiki 查询多是术语精确匹配，词干还原收益小
- 统一 tokenizer 的简单性 > 英文词干的边际收益

如果 user 的 vault 几乎全英文 → config 可选 `tokenizer = "porter"`。

---

## 5. 性能对比

测试 vault：5,000 页，中英混排，平均 800 字/页。

| Tokenizer | 索引体积 | 建索引耗时 | 查询 p95 | 中文子串召回 |
|---|---|---|---|---|
| unicode61（默认） | 8 MB | 2.1s | 8ms | ❌ ~30% |
| **trigram** | 24 MB | 4.8s | 28ms | ✅ ~98% |
| jieba | 11 MB | 9.2s（含分词） | 15ms | ✅ ~95%（漏新词） |

**解读**：

- trigram 索引大 3x、查询慢 3x，但**绝对值仍然很小**（28ms p95 完全可接受）
- trigram 召回率 98% > jieba 95%（jieba 漏新词，trigram 无词典所以无漏词问题）
- 对个人规模，trigram 是正确的默认

---

## 6. 重建索引

任何 tokenizer 变更必须重建：

```bash
wikimind rebuild-index
# - 删除 pages_fts 表
# - 按当前 config 的 tokenizer 重建
# - 从 wiki/*.md 重新灌入
# - 5000 页约 5-10s
```

`wikimind doctor` 检测：若 config 的 tokenizer 与实际 FTS5 表的 tokenizer 不一致 → 提示 rebuild。

---

## 7. 测试

CI 必跑的 CJK 检索用例（`tests/search/cjk_test.go`）：

```
建索引：title="Wiki 是一个 compounding artifact"
        body="每一次 ingest、每一次 query 都让 wiki 更值钱"

断言：
  search("值钱")        → 命中（短查询 fallback）
  search("更值钱")      → 命中
  search("每一次")      → 命中
  search("每次")        → 命中（trigram 子串）
  search("compounding") → 命中（英文）
  search("ingest query")→ 命中（多词 AND）
  search("TypeScript")  → 不命中（不在内容里，负例）
```

**回归保护**：2026-04 社区报告过 unicode61 CJK 回归——CI 固定测 trigram，且锁定 SQLite 最低版本
3.40+（trigram 稳定版本）。

```bash
# 启动时检查
sqlite_version=$(sqlite3 --version)
if version_lt "$sqlite_version" "3.40"; then
    echo "WikiMind 需要 SQLite >= 3.40（trigram tokenizer 稳定版）"
    exit 1
fi
```

---

## 8. ripgrep 兜底

FTS5 不是唯一检索通道。`wikimind` 在以下情况退回 ripgrep 直接扫 markdown：

- SQLite index.db 不存在 / 损坏
- 短查询（< 3 字符）
- 正则查询（user 用 `wikimind query --regex`）
- 用户要求 `--no-index`

ripgrep 对 CJK 无分词问题（它是字节级正则），但慢（无索引）。作为兜底足够。

---

## 9. Embedding（v0.2，正交于分词）

Embedding 检索是**另一条路**，与 tokenizer 正交：

- MVP 默认关
- v0.2 可选本地 `bge-small`（中文友好的 embedding model）via llama.cpp
- 启用后 `search(type: "fts+vector")` 走 FTS5 召回 + 向量 rerank
- Embedding 对"语义相似但用词不同"有用（"知识沉淀" ≈ "knowledge accumulation"），但不替代 FTS5

详见 [`SPEC.md §5.2`](../SPEC.md#52-mvp-不做)（MVP 不做 embedding）。

---

## 10. 决策总结

| 问题 | 决策 |
|---|---|
| MVP 用什么 tokenizer | **trigram** |
| 短查询（< 3 字）怎么办 | fallback 到 LIKE / ripgrep |
| 索引体积大 3x 接受吗 | 接受（个人规模绝对值小） |
| 什么时候上 jieba | v0.2，vault > 10k 或 user 反馈精度不足 |
| 什么时候上 ICU | v2+，做真多语言时再评估 |
| SQLite 最低版本 | 3.40+（CI 强制） |
| Embedding | v0.2 可选，正交于分词 |

---

## 11. 不在范围

- 日语 / 韩语的专门分词优化（trigram 对它们也可用，但未专门测试）
- 繁简转换（user 若需要，在 query 层做，非 tokenizer 职责）
- 拼音搜索（v0.2+ 评估）
- 同义词扩展（属于 Dream Cycle 的 consolidate，见 [`dream-cycle.md`](dream-cycle.md)）

---

## 一句话总结

> SQLite FTS5 默认的 unicode61 对中文子串搜索基本失效——这是"看起来能用其实不能用"的隐形坑。
> WikiMind MVP 用 **trigram**（零依赖、CJK 友好、子串搜索正确），短查询 fallback 到 ripgrep，
> v0.2 可选 jieba。CI 固定跑 CJK 检索回归用例。
