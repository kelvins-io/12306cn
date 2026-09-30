.PHONY: help tidy build-backend build-backend-linux test-backend run-backend deps-frontend build-frontend run-frontend up down logs restart clean

help:
	@echo "可用目标:"
	@echo "  make tidy                 - go mod tidy"
	@echo "  make build-backend        - 编译后端 (本机)"
	@echo "  make build-backend-linux  - 交叉编译 linux/amd64 供 Docker"
	@echo "  make test-backend         - 后端单元测试"
	@echo "  make run-backend          - 本地运行后端"
	@echo "  make deps-frontend        - 安装前端依赖"
	@echo "  make build-frontend       - 构建前端"
	@echo "  make run-frontend         - 本地启动前端 Vite"
	@echo "  make up                   - 本地构建产物 + docker compose 启动"
	@echo "  make down                 - 停止并移除容器"
	@echo "  make logs                 - 查看 compose 日志"
	@echo "  make restart              - 重启 compose"
	@echo "  make clean                - 清理构建产物"

tidy:
	cd backend && go mod tidy

build-backend:
	cd backend && go build -o bin/server ./cmd/server && go build -o bin/query ./cmd/query \
		&& go build -o bin/inventory ./cmd/inventory && go build -o bin/gateway ./cmd/gateway

build-backend-linux:
	cd backend && CGO_ENABLED=0 GOOS=linux go build -o bin/server ./cmd/server \
		&& CGO_ENABLED=0 GOOS=linux go build -o bin/query ./cmd/query \
		&& CGO_ENABLED=0 GOOS=linux go build -o bin/inventory ./cmd/inventory \
		&& CGO_ENABLED=0 GOOS=linux go build -o bin/gateway ./cmd/gateway

test-backend:
	cd backend && go test ./...

run-backend:
	cd backend && \
	DATABASE_DSN='host=localhost user=postgres password=postgres dbname=ticket port=12345 sslmode=disable TimeZone=Asia/Shanghai' \
	REDIS_ADDR=localhost:16380 \
	go run ./cmd/server

deps-frontend:
	cd frontend && npm install

build-frontend:
	cd frontend && npm run build

run-frontend:
	cd frontend && npm run dev

up: build-backend-linux build-frontend
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

restart:
	docker compose restart

clean:
	rm -rf backend/bin frontend/dist
	docker compose down -v || true
