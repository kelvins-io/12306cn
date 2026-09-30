# 12306cn

基于 Go + Vue3 的铁路票务演示系统，参考 12306 核心架构要点实现：

- **查询与交易分离（CQRS）**：余票查询可走 Redis 短缓存；下单占座走强一致事务
- **区间库存位图**：座位按原子区间占用，支持席位复用
- **车次级锁**：Redis `SETNX` + Postgres 行锁，降低超卖
- **预占座**：下单 pending → 支付出票 / 超时释放
- **候补**：退票后按序尝试兑现

> 本项目为教学/演示用途，非官方 12306，请勿用于生产或真实售票。

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.22+、Gin、GORM、JWT |
| 数据库 | PostgreSQL 13 |
| 缓存/锁 | Redis 7 |
| 前端 | Vue 3、Vite、Element Plus、Pinia、Vue Router、Axios |
| 部署 | Docker Compose、Nginx、Makefile |

## 功能清单

- 用户注册 / 登录（JWT）
- 车站检索、车次余票查询
- 乘车人管理
- 下单占座、模拟支付、取消、退票
- 候补登记与退票后兑现（演示自动支付）

## 快速启动（Docker）

默认 Dockerfile 使用本机交叉编译产物 + 已有基础镜像，避免拉取 `golang`/`node` 构建镜像：

```bash
make up
# 等价于: make build-backend-linux build-frontend && docker compose up -d --build
```

若网络允许完整多阶段构建，可改用 `backend/Dockerfile.build` 与 `frontend/Dockerfile.build`。

启动后访问：

- 前端：http://localhost:5473
- API 网关：http://localhost:10004/api/health
- 交易 API：http://localhost:10001/api/health
- 查询 API：http://localhost:10002/api/health
- 库存 API：http://localhost:10003/internal/v1/health
- Postgres：localhost:12345
- Redis：localhost:16380

演示账号：

| 账号 | 密码 | 角色 |
|------|------|------|
| demo | 123456 | 普通用户 |
| admin | admin123 | 运营管理员 |
| station | station123 | 车站核验 |

### 架构能力

- 选座 / 票种计价 / 区段配额 / 排队限流 / 改签候补
- **运营配置 API**：配额、放票策略、风控黑名单、**席位封锁**、**按日封锁**、**放票波次**（`/api/admin/*`）
- **改签规则**：最多 2 次、发车前 2 小时截止、高低改差价
- **中转换乘（简版）**：同日一站换乘查询与两程联下单
- **分时放票多波次**：按席别/区段累计放票比例
- **余票投影**：`remain_projections` 读模型，查询优先读投影
- **验证码风控**：登录/下单需算术验证码
- **支付抽象**：`PAYMENT_PROVIDER=mock`，支付流水表
- **候补 MQ**：Redis List 异步兑现
- **API 网关**：路由 + **自适应下单限流**（按后端延迟调速）
- **取票核验**：支付后生成取票码；车站账号核验
- query / inventory / backend / gateway 拆分；事件刷新读模型

停止：

```bash
make down
```

查看日志：

```bash
make logs
```

## 本地开发

### 依赖服务

至少需要 PostgreSQL 13 与 Redis，可用 compose 只起中间件：

```bash
docker compose up -d postgres redis
```

### 后端

```bash
make tidy
make run-backend
# API: http://localhost:8080
```

环境变量（可选）：

| 变量 | 默认 | 说明 |
|------|------|------|
| `SERVER_PORT` | `8080` | 服务端口 |
| `DATABASE_DSN` | localhost postgres/ticket | GORM DSN |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `JWT_SECRET` | 开发默认值 | JWT 密钥 |
| `JWT_EXPIRE_HOURS` | `72` | Token 有效期 |
| `ORDER_HOLD_MINUTES` | `15` | 未支付订单保留分钟 |
| `PAYMENT_PROVIDER` | `mock` | 支付实现 |
| `BOOK_RATE_MIN` / `BOOK_RATE_MAX` | `1` / `20` | 网关自适应限流上下限 |

### 前端

```bash
make deps-frontend
make run-frontend
# http://localhost:5173 ，/api 代理到 8080
```

## API 概览

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/api/auth/register` | 否 | 注册 |
| POST | `/api/auth/login` | 否 | 登录 |
| GET | `/api/stations?q=` | 否 | 车站 |
| GET | `/api/tickets?from=&to=&date=` | 否 | 余票查询 |
| GET/POST/DELETE | `/api/passengers` | 是 | 乘车人 |
| POST/GET | `/api/orders` | 是 | 下单/列表 |
| POST | `/api/orders/:id/pay` | 是 | 支付 |
| POST | `/api/orders/:id/cancel` | 是 | 取消 |
| POST | `/api/orders/:id/refund` | 是 | 退票 |
| POST | `/api/orders/:id/reschedule` | 是 | 改签 |
| POST/GET | `/api/waitlist` | 是 | 候补 |
| GET/POST | `/api/admin/*` | admin | 运营配置 |
| GET/POST | `/api/verify/ticket` | station/admin | 取票核验 |
| GET | `/api/gateway/stats` | 否 | 网关自适应限流状态 |

请求头：`Authorization: Bearer <token>`

统一响应：

```json
{ "code": 0, "message": "ok", "data": {} }
```

## 目录结构

```
.
├── backend/
│   ├── cmd/server/          # 交易 API
│   ├── cmd/query/           # 查询微服务
│   ├── cmd/inventory/       # 库存微服务
│   ├── cmd/gateway/         # API 网关 + 自适应限流
│   ├── internal/
│   │   ├── services/
│   │   │   ├── inventory/   # 区间库存 + 区段配额
│   │   │   ├── query/       # 读模型
│   │   │   ├── payment/     # 支付抽象
│   │   │   ├── waitmq/      # 候补 MQ
│   │   │   ├── adaptive/    # 自适应限流
│   │   │   ├── queueing/    # 订票排队限流
│   │   │   └── service.go   # 订单/改签/候补编排
│   ├── Dockerfile
│   ├── Dockerfile.query
│   ├── Dockerfile.inventory
│   └── Dockerfile.gateway
├── frontend/
├── docker-compose.yml
├── Makefile
└── README.md
```

## 库存模型说明

车次站点序列 `A-B-C-D` 形成原子区间 `AB,BC,CD`。每个座位用 `occupancy` 位图标记区间是否占用。

- 购买 `B→D`：需要 `BC|CD` 位为空，然后置位
- 购买 `A→B`：只占用 `AB`，与 `B→D` 不冲突（席位复用）

下单流程：Redis 车次锁 → 事务内 `SELECT FOR UPDATE` 选座 → 乐观版本更新 → 写订单票面。

## Makefile

```bash
make help
make up / down / logs
make build-backend / test-backend / run-backend
make build-frontend / run-frontend
```

## 许可证

仅供学习交流。
