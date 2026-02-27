# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

这是一个基于 Go 的加密项目（CryptoTips v1.4.0），当前核心定位是：
- 比特币价格测量
- 比特币价格预测
- 实时给出买卖点提示

系统以数据采集与指标分析为基础，前端提供可视化，后端提供 API。

## 关键事实（必须优先遵循）

1. **前端核心页面只有两个：**
   - K线页（K线 + 成交量 + 成交额）
   - 贪婪指数页（当前指数 + 历史数据）

2. **生产运维核心 systemd 服务：**
   - `goapi.service`
   - `fetchklines.service`
   - `feargreed.service`

3. **当用户说“重启 goapi 服务”等运维指令时：**
   - 直接执行对应 systemctl 命令（如 `sudo systemctl restart goapi.service`）

## 常用命令

```bash
# 编译
go build -o cryptotips ./

# 依赖管理
go mod tidy

# 前台运行（开发调试）
./cryptotips goapi --daemon=false

# 后台运行（默认 daemon 模式）
./cryptotips goapi

# 启动依赖服务（MySQL + Redis）
docker-compose -f dockers/docker-compose.yml up -d

# 运行测试
go test ./indicator/... -run TestSMA -v
go test ./... -v
```

## 配置

**加载顺序：**
1. `.env`（从 `env.template` 复制，设置 `APP_ENV=dev` 或 `APP_ENV=pro`）
2. `./config/config.{dev|pro}.yaml`（从 `config.template.yaml` 复制）

`config/config.*.yaml` 和 `.env` 已被 `.gitignore` 排除，不提交到版本库。

### 端口差异（重要）

- 模板配置（常用于开发）里 `apiservice.port` 常见为 `1024`
- 当前生产配置 `config/config.pro.yaml` 中 `apiservice.port` 为 `90`

联调/排障时请先确认当前 `APP_ENV` 和实际端口。

## 架构

### 主启动链路

```text
main.go -> lib.LoadConfig() -> cmd.Execute()
                                  ↓
                           cmd/goapi.go（CLI入口）
                                  ↓
                        app/goapi.go（Gin REST API）
```

- `cmd/`：Cobra CLI 命令，处理 daemon 启停与信号
- `app/`：HTTP 服务实现与业务逻辑（Gin）
- `lib/`：配置、DB/Redis、日志、daemon、HTTP 工具等
- `models/`：GORM 模型
- `indicator/`：技术指标计算（SMA/EMA/MACD/RSI/ATR/Bollinger/Sharpe180 等）
- `frontend/`：React + TS + lightweight-charts

### 数据主链路（快速理解系统）

```text
Binance/外部数据源
   -> fetchklines / feargreed 采集
   -> MySQL(crypto_kline, fear_greed_index, indicators...)
   -> goapi 提供 /api/*
   -> frontend 展示（K线页 / 贪婪指数页）
```

## 关键设计

### MySQL 关键表
- `crypto_kline`：K线数据，唯一索引 `(symbol, interval, open_time)`
- `indicators`：技术指标结果
- `strategy_log`：策略决策日志
- `trades`：交易记录（含止损/止盈/P&L）
- `fear_greed_index`：贪婪恐慌指数历史数据

### Redis 常见 key
```text
market_state:{symbol}
order_lock:{symbol}:{side}
drawdown_pause
consecutive_loss:{symbol}
```

### 指标包约定
每个指标通常提供两类函数：
- 返回完整序列（如 `SMA()`）
- 返回最新值（如 `SMALast()`）

## 日志输出

- `./logs/goapi.log`
- `./logs/fetchklines.log`
- `./logs/feargreed.log`

常见 PID / error 文件位于 `./logs/`。

## Frontend

### 技术栈
- React 18 + TypeScript 5.5
- Vite 5.4
- lightweight-charts 4.2
- CSS Modules

### 页面与目录重点
- `frontend/src/components/KLineChart.tsx`：K线 + 成交量 + 成交额
- `frontend/src/components/FearGreedPage.tsx`：贪婪指数仪表盘 + 历史图
- `frontend/src/api/klines.ts`：`/api/klines` 等接口
- `frontend/src/api/feargreed.ts`：`/api/feargreed/history`

### 开发代理
- Vite 开发时将 `/api/*` 代理到后端（见 `frontend/vite.config.ts`）
- 生产建议通过 Nginx 反代 `/api` 到后端服务端口

## systemd 运维速查（生产）

```bash
# 查看状态
sudo systemctl status goapi.service
sudo systemctl status fetchklines.service
sudo systemctl status feargreed.service

# 重启服务
sudo systemctl restart goapi.service
sudo systemctl restart fetchklines.service
sudo systemctl restart feargreed.service

# 查看日志
sudo journalctl -u goapi.service -n 200 --no-pager
sudo journalctl -u fetchklines.service -n 200 --no-pager
sudo journalctl -u feargreed.service -n 200 --no-pager
```

## 定时采集口径说明

- `fetchklines` 的增量周期由 `kline_sync.interval_minutes` 控制（或命令行参数覆盖）。
- `feargreed` 的采集周期以配置 `feargreed.interval_hours` 为准。
- 若代码注释/文档与配置不一致，**以当前生效配置为准**。

## 安全与协作注意

- 配置文件中可能包含敏感信息（API Key、TG Token、2captcha Key），禁止外传。
- 变更生产配置前先备份，改后必须做可用性验证（服务状态 + API + 页面）。
- 优先最小权限原则，避免不必要的高权限运行。