# cryptotips/workspace

该目录用于集中管理 YouTube 研究与因子工程资产，遵循“条理化、规则化”。

## 目录说明

- `youtube/`：预留给 YouTube 任务专用子目录（后续可继续细分 raw/chunks/transcripts）。
- `scripts/`：下载、转写、分析、因子构建脚本。
- `data/`：轻量数据与样例索引（可提交）。
- `analysis/`：分析结果与因子数据集（可提交，注意体积）。
- `reports/`：阶段报告与评估文档。

## 运行建议（标准流程）

1. 数据拉取：`python workspace/scripts/factor_data_pull_v1.py`
2. 因子构建：`python workspace/scripts/traderchenge_factor_build_v1.py`
3. 内容评估：`python workspace/scripts/traderchenge_content_eval_v2.py`
4. 查看报告：`workspace/reports/*.md`

## 提交规范

- 提交：脚本、报告、轻量 json/csv、分析产物。
- 不提交：大音频、chunks、cookies、密钥、超大原始缓存。
- 所有新增脚本应保持可复现（固定输入输出路径）。

## 命名规范

- 脚本：`*_v1.py`（迭代升级时 `v2/v3`）
- 报告：`*_v1.md`
- 指标定义：`*_definitions_*.json`
- 评估指标：`*_metrics_*.json`
