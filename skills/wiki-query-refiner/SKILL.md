---
name: wiki-query-refiner
description: "自动将模糊用户问题扩展为多维度精准查询，提升 Wiki 问答质量"
---

# Wiki 查询优化器

当用户的问答请求过于模糊、宽泛或可能匹配过多无关结果时触发。

## 前置条件

- 服务已启动：`http://localhost:8080`
- 用户提出了一个问题但表述不够精准

## 工作流程

### 1. 判断是否需要扩展

**不需要扩展的问题（直接查询即可）：**
- 包含具体 API 名/函数名：「`SendOrder` 接口的参数是什么？」
- 包含具体文件名：「`OrderService.go` 里的错误处理怎么写的？」
- 包含具体业务术语：「骑手位置反馈支持哪些配送平台？」
- 问题长度 > 15 个字符且包含多个关键词

**需要扩展的问题特征：**
- 过于简短：「这个项目干嘛的」「订单怎么回事」
- 范围过大：「帮我介绍一下」「有什么功能」
- 可能跨模块：「支付流程是怎样的」（可能涉及 APP 端 + 后端 + 第三方）
- 单关键词：「配送」「订单」「用户」

### 2. 问题扩展策略

#### 策略 A：关键词拆分 + 多维并行查询

```
原始问题: "配送"

扩展为以下子查询（并行发送）：
  Q1: "配送 订单 分配规则"     → 匹配 order 模块的分配逻辑
  Q2: "配送 骑手 位置上报"      → 匹配位置反馈服务
  Q3: "配送 配送员 管理"        → 匹配 APP 端配送管理页面
  Q4: "配送 多平台 SDK 对接"    → 匹配各平台适配器

综合 4 个结果 → 去重合并 → 输出结构化答案
```

#### 策略 B：上下文补全

```
如果用户指定了 project 但没指定 sub_project：
  "招财快送的订单" 
    → 自动拆解为：
      - yl-delivery-service 的订单 controller/service/model (后端视角)
      - zcks-app 的订单相关页面 (APP端视角)
      - zcks-server 的订单 gRPC 接口 (服务间调用视角)
    → 分别查 3 个子项目 → 综合回答
```

#### 策略 C：技术栈感知转换

```
用户用自然语问："前端怎么提交订单"
  → 自动转换为技术术语查询：
    Q1: "manualAddOrder API 调用"
    Q2: "手动添加订单 弹窗 组件"
    Q3: "order add 提交 参数"

用户问："数据库怎么存订单的"
  → 转换为：
    Q1: "order model 数据表结构"
    Q2: "订单 MySQL 表定义"
    Q3: "OrderSchema 字段说明"
```

### 3. 执行多路查询

```bash
# 并行发起多次查询（用 & 后台执行）
for q in "${expanded_queries[@]}"; do
  curl -s -X POST http://localhost:8080/api/query \
    -H "Content-Type: application/json" \
    -d "{\"question\":\"$q\",\"project\":\"$project\",\"sub_project\":\"$sub\"}" \
    > /tmp/query_result_$$.json &
done
wait

# 合并去重
python3 -c "
import json, sys, glob
all_sources = {}
answers = []
for f in glob.glob('/tmp/query_result_*.json'):
    d = json.load(open(f))
    if d.get('answer') and len(d['answer']) > 20:
        answers.append(d['answer'])
    for s in d.get('sources', []):
        all_sources[s] = True
print(json.dumps({'answers': answers, 'unique_sources': list(all_sources.keys())}, ensure_ascii=False))
"
```

### 4. 综合输出格式

```
=== 综合问答结果 ===

原始问题: {user_question}
扩展方向: {N} 个维度 ({列出各维度关键词})

【综合答案】
{基于多个子查询结果合并的去重、结构化答案}

【信息来源】({M} 个唯一来源)
  - {source_1} (来自: {sub_project})
  - {source_2} (来自: {sub_project})

【各维度详情】(如用户需要展开)
  维度1 "{dim1}": {该维度的独立答案摘要}
  维度2 "{dim2}": {该维度的独立答案摘要}
```

## 扩展模板库

针对「招财快送」项目的预置扩展规则：

| 用户关键词 | 扩展为 |
|-----------|--------|
| 订单 | order controller + 订单状态枚举 + manualAddOrder + zcks-app订单页 + zcks-server SendOrder |
| 配送 | 骑手位置上报(10平台SDK) + delivery模块 + 配送员管理 + 配送设置 |
| 店铺 | shop model + 店铺列表 + 店铺配置API + APP店铺选择器 |
| 收据 | receipt 模块 + 打印机对接 + 小票模板 |
| 支付 | 支付回调 + 订单金额计算 + tip(小费) |
| 用户 | user store + 登录注册 + 权限管理 |
| 统计 | stat 页面 + 数据看板 + 报表导出 |

通用扩展规则（适用于任何项目）：
- "XXX 怎么做" → "XXX 流程" + "XXX 实现" + "XXX 步骤" + "XXX API"
- "XXX 有什么" → "XXX 列表" + "XXX 类型" + "XXX 枚举" + "XXX 配置"
- "XXX 为什么" → "XXX 原因" + "XXX 设计" + "XXX 背景" + "XXX issue"

## 注意事项

1. **不要过度扩展** — 如果原问题已经足够具体（>20字符+包含专有名词），直接查询不扩展
2. **控制并发数** — 最多拆成 5 个子查询，避免 LLM API 调用爆炸
3. **合并时去重** — 不同子查询可能返回相同 source，按 Source 文件名去重
4. **标记来源归属** — 明确告诉用户每个信息片段来自哪个子项目
5. **成本意识** — 每次扩展 = N 倍 LLM Chat 调用成本。对简单问题不要滥用此 skill
