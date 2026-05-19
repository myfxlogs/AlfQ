# Frontend — ALFQ Web SPA

React 前端，通过 Connect RPC 与 trading-core 通信。

## 目录

```
frontend/
├── src/            # 源代码
│   ├── gen/        # buf generate 产物（不手改）
│   │   └── alfq/v1/*.ts
├── package.json
├── vite.config.ts
├── tsconfig.json
├── tailwind.config.ts
└── README.md
```

## 技术栈

- React 19 + TypeScript 5.4+ + Vite 8
- shadcn/ui + Tailwind CSS + TanStack Query + Zustand
- `@connectrpc/connect-web` + `@bufbuild/protobuf`（V2，零外部依赖）

## 通信

所有请求走 Connect RPC（HTTP/JSON）。流式推送走 Server Streaming（SSE）。

Proto 代码生成：`cd backend/proto && buf generate` 同时产出 Go 后端 + TS 前端类型。
前端产物位于 `src/gen/alfq/v1/`，包含 `*_pb.ts`（message 类型）和 `*_connect.ts`（客户端 stub）。
