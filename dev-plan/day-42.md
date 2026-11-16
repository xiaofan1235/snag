# Day 42 —— UI：Finding + Evidence + Approval + Coverage

> 里程碑：M6 | 依赖：Day 40 | 产出：P0 页面组 2
> 对照 UI 文档：§16.7/16.9/16.10/16.8、D6

## 0. 今天做什么
Finding 详情（事实/推断分区 + 点击即证据）、Evidence（脱敏 + Reveal）、Approval 中心（动作 Diff）、Coverage 矩阵。

## 1. 进度位置
第 42 天。用户"审批 + 看发现 + 看证据 + 看覆盖"的核心面。

## 4. 设计细节
- Finding（§16.7/D6）：事实区/推断区视觉分开；baseline/variant 并排 diff；点击结论→右栏 CEBS 字节高亮（§3.4）。
- Evidence（§16.9）：默认脱敏投影；Raw 需 Reveal（短时授权 Modal）；HTML 不执行。
- Approval（§16.10）：展示规范化动作+对象归属+副作用+策略原因+差异；Approve Once/Deny/Deny and Cancel Related；不批量批准高风险。
- Coverage（§16.8）：矩阵、单元格下钻 Action+Evidence。

## 5. 分步开发
1. Finding 详情 + 点击即证据。
2. Evidence 脱敏 + Reveal。
3. Approval 中心。
4. Coverage 矩阵。

## 6. 完成效果（DoD）
- 从 Finding 可跳 Verifier/Action/Exchange/Evidence；审批显示规范化动作；Reveal 需授权。

## 7. 怎么测试
- Playwright：Finding→证据跳转；审批流；Reveal 授权。
- 命令：`pnpm test:e2e`
- 对照：UI 验收清单 2/3。
