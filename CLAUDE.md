# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

这是一个基于Go语言的加密货币自动交易系统（CryptoTips v1.4.0），支持市场状态驱动的交易策略（BULL/BEAR/NEUTRAL），集成MySQL和Redis，提供REST API服务。

## 常用命令

```bash
# 编译
go build -o cryptotips ./

# 依赖管理
go mod tidy

# 前台运行（开发调试）
./cryptotips goapi --daemon=false

# 后台运行（默认daemon模式）
./cryptotips goapi

# 启动依赖服务（MySQL + Redis）
docker-compose -f demands/docker-compose.yml up -d

# 运行单个测试
go test ./indicator/... -run TestSMA -v
go test ./... -v
```

## 配置

**加载顺序：**
1. `.env` 文件（需从 `env.template` 复制，设置 `APP_ENV=dev` 或 `APP_ENV=pro`）
2. `./config/config.{dev|pro}.yaml`（从 `config.template.yaml` 复制）

`config/config.*.yaml` 和 `.env` 已被 `.gitignore` 排除，不提交到版本库。

## 架构

### 分层结构

```
main.go → lib.LoadConfig() → cmd.Execute()
                                    ↓
                             cmd/goapi.go（CLI入口）
                                    ↓
                          app/goapi.go（Gin REST API）
```

- **`cmd/`** - Cobra CLI命令定义，处理daemon启动/信号
- **`app/`** - Gin框架的HTTP服务实现，路由在 `app/goapi.go`
- **`lib/`** - 通用工具库：配置加载、DB/Redis初始化、日志、HTTP客户端、daemon管理等
- **`models/`** - GORM数据模型，4个核心表：`crypto_kline`、`indicators`、`strategy_log`、`trades`
- **`indicator/`** - 纯函数技术指标计算（SMA/EMA/MACD/RSI/ATR/Bollinger/Sharpe180）

### 关键设计

**MySQL表：**
- `crypto_kline`：K线数据，唯一索引 `(symbol, interval, open_time)`，使用MD5去重+BatchUpsert
- `indicators`：技术指标，包含MA200/EMA/RSI/MACD/ATR/布林带/Sharpe180
- `strategy_log`：策略决策日志（BULL/BEAR/NEUTRAL状态判断）
- `trades`：交易记录，含止损/止盈/P&L

**Redis key结构：**
```
market_state:{symbol}         # 市场状态
order_lock:{symbol}:{side}    # 下单防重复锁
drawdown_pause                # 回撤暂停标记
consecutive_loss:{symbol}     # 连续亏损计数
```

**indicator包约定：** 每个指标提供两个版本，如 `SMA()` 返回完整数组，`SMALast()` 仅返回最新值。

### 日志输出

- 文件：`./logs/goapi.log`（JSON格式）
- PID：`./logs/app.goapi.pid`
- 错误：`./logs/app.goapi.error`

## 技术栈

### 后端（Go）

| 用途 | 库 |
|------|----|
| CLI框架 | `spf13/cobra` |
| HTTP框架 | `gin-gonic/gin` |
| ORM | `gorm.io/gorm` + MySQL驱动 |
| Redis | `redis/go-redis/v9` |
| 日志 | `sirupsen/logrus` |
| 配置 | `spf13/viper` + `joho/godotenv` |
| Daemon | `sevlyar/go-daemon` |
| TLS客户端 | `bogdanfinn/tls-client` |
| 系统监控 | `shirou/gopsutil/v3` |
| 定时任务 | `go-co-op/gocron v1.37.0` |

### 前端（TypeScript/React）

| 用途 | 库/工具 |
|------|---------|
| UI框架 | `react` 18.3 + `react-dom` |
| 构建工具 | `vite` 5.4 + `@vitejs/plugin-react` |
| 类型系统 | `typescript` 5.5 |
| 图表库 | `lightweight-charts` 4.2（TradingView风格） |
| 样式方案 | CSS Modules（组件级隔离） |

## Frontend

### 目录结构

```
frontend/
├── src/
│   ├── api/            # HTTP 请求封装
│   │   ├── klines.ts       # K线数据接口（/api/klines, /api/klines/meta）
│   │   └── feargreed.ts    # 恐慌贪婪指数接口（/api/feargreed/history）
│   ├── components/     # React 组件
│   │   ├── Toolbar.tsx         # 顶部工具栏：品牌 + 页面专属控件（Symbol/Interval等）
│   │   ├── Sidebar.tsx         # 左侧纵向菜单：页面导航（新增页面在此扩展）
│   │   ├── KLineChart.tsx      # K线图表：蜡烛图+成交量+成交额三层图
│   │   └── FearGreedPage.tsx   # 恐慌指数页：仪表盘+历史折线图
│   ├── types/          # TypeScript 类型定义
│   │   ├── kline.ts        # KLineItem, KLinesData, ApiResponse<T>
│   │   └── feargreed.ts    # FearGreedItem, FearGreedHistoryData
│   ├── App.tsx         # 主应用：页面切换、数据加载、状态管理
│   └── main.tsx        # React 入口
├── index.html
├── vite.config.ts      # 开发代理：/api → http://localhost:1024
└── package.json
```

### 常用命令

```bash
cd frontend

# 安装依赖
npm install

# 开发服务器（端口 5173，自动代理 /api 到后端）
npm run dev

# 类型检查 + 生产构建（输出到 dist/）
npm run build

# 预览构建产物
npm run preview
```

### 页面布局结构

```
┌──────────────────────────────────────┐
│  📈 Crypto Viewer  │ 页面专属控件区   │  ← Toolbar（顶部全宽，品牌 + 控件）
├───────────┬──────────────────────────┤
│ Sidebar   │                          │
│ 📈 K 线图 │    内容区域（Main）       │  ← Sidebar（左，152px）+ Main（右，flex:1）
│ 😨 贪婪指数│                          │
└───────────┴──────────────────────────┘
```

- `Toolbar`：仅包含品牌 + 当前页面的专属控件（如 K线图页显示 Symbol/Interval/刷新）
- `Sidebar`：左侧纵向导航菜单，新增页面只需在 `NAV_ITEMS` 数组追加一项

### 页面功能

- **K线图页面**：三层堆叠图表（蜡烛图 OHLC / 成交量 / 成交额 USDT），支持 Symbol 和 Interval 选择
- **恐慌贪婪指数页面**：指针仪表盘（5级分类：Extreme Fear→Extreme Greed）+ 历史折线图

### 开发代理配置

开发环境下 Vite 将 `/api/*` 请求代理到 `http://localhost:1024`，生产环境通过 `VITE_API_BASE_URL` 环境变量配置后端地址（可选，默认使用相对路径由 Nginx 等反向代理处理）。

