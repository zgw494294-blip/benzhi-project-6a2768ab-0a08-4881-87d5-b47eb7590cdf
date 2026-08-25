# BENZHI_README

基于 Go 实现的seed-vault-admission Web 项目，一款后端服务，已完整实现古树种子迁地保育材料从来源草拟、分装登记、质量判定、异常整改和独立复核，到清单冻结、不可变凭据签发与核验的浏览器工作台，并提供摘要链日志、原子快照和真实 HTTP 自检。

## 项目说明
- 项目：benzhi-project-6a2768ab-0a08-4881-87d5-b47eb7590cdf
- 项目用途：已完整实现古树种子迁地保育材料从来源草拟、分装登记、质量判定、异常整改和独立复核，到清单冻结、不可变凭据签发与核验的浏览器工作台，并提供摘要链日志、原子快照和真实 HTTP 自检。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-6a2768ab-0a08-4881-87d5-b47eb7590cdf-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-6a2768ab-0a08-4881-87d5-b47eb7590cdf-arm64 linux/arm64
docker run -it benzhi-project-6a2768ab-0a08-4881-87d5-b47eb7590cdf-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
