# BENZHI_README

基于 Go 实现的stage-rigging-release Web 项目，一款后端服务，面向剧场舞台技术团队的吊挂装置装台验收与演出放行系统，完整实现批次维护、吊点锁定、载荷试验、偏差整改复测、技术复核、冻结摘要、不可变凭据和同源浏览器工作台。

## 项目说明
- 项目：benzhi-project-b9ffb70e-2f8e-429a-9188-86ed976a34ed
- 项目用途：面向剧场舞台技术团队的吊挂装置装台验收与演出放行系统，完整实现批次维护、吊点锁定、载荷试验、偏差整改复测、技术复核、冻结摘要、不可变凭据和同源浏览器工作台。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19137 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-b9ffb70e-2f8e-429a-9188-86ed976a34ed-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-b9ffb70e-2f8e-429a-9188-86ed976a34ed-arm64 linux/arm64
docker run -it benzhi-project-b9ffb70e-2f8e-429a-9188-86ed976a34ed-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19137 -selfcheck`
