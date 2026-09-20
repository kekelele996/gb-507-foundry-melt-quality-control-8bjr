请生成 `foundry-melt-quality-control`「铸造炉次成分与质量判定」Go 全栈项目，面向铸造工厂管理炉次、炉前取样、化验结果和出炉质量决定。项目聚焦金属制造质量状态机，不做采购、仓库、订单或成本核算。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`Furnace`（炉台和能力）、`Heat`（炉次与熔炼阶段）、`ChemicalSample`（取样与成分）、`QualityDecision`（合格/返炉/报废）必须贯穿数据库、Go model/service/handler 和前端。

### 核心页面

`/furnaces` 炉台总览；`/heats` 炉次工作台；`/samples` 化验结果；`/decisions` 质量判定；`/audit` 审计。`HeatStateBadge` 在炉次和判定页共用，`ChemistryTable` 在样本和详情页共用。

### 横切关注点

RBAC 权限联动后端中间件、前端守卫和按钮；质量决定审计记录状态迁移与操作者；全局错误处理、request ID、限流独立实现。

### 共享枚举/组件

定义 `HeatState`（charged/melting/sampling/hold/accepted/rejected）与 `DecisionType`（accept/remelt/scrap）。共享 `StatusBadge`、`ChemistryTable`、`EmptyState`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 React 18 + TypeScript + Vite + Material UI；后端 Go 1.22 + Gin + GORM；PostgreSQL + Redis。目标 3000–4200 行、30–42 个 `.go` 文件，功能文件不少于 20 个。

### 文件结构强制清单

前端必须拆 `api/stores/types/components/common/hooks/pages/router/utils`；后端必须拆 `model/dto/repository/service/handler/router/middleware/constants/util`。README 列出枚举所有出现位置。

### 结构红线

严禁合并职责到单一文件；质量状态机必须通过多个实体和层次表达。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: foundry-melt-quality-control`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=foundry-melt-quality-control`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18507`、后端端口 `19507`；使用 Nginx `/api` 反代、数据库 healthcheck、命名卷和 `condition: service_healthy`，提供真实 `/healthz`、Git 初始化。
