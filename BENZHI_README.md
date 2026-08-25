基于 Go 实现的 JSON 处理 CLI 与 Web 项目，一套提供解析、序列化、Schema 校验、命令行处理和浏览器调试能力的工具。

# jsonx 项目说明

`jsonx` 是一个不依赖 `encoding/json` 核心实现的 JSON 解析、序列化与校验项目，包含 `jsonctl` 命令行工具和离线 Web Playground。

## 环境要求

- Go 1.22 或更高版本
- Docker（仅容器打包与验证需要）

## 构建与测试

```bash
go build ./...
go vet ./...
go test ./... -count=1
go test -race ./...
```

## 运行

运行命令行工具：

```bash
go run ./cmd/jsonctl format example.json -i 2
```

也可以从标准输入读取 JSON：

```bash
printf '%s' '{"name":"jsonx"}' | go run ./cmd/jsonctl format - -i 2
```

启动 Web Playground：

```bash
go run ./cmd/webui -addr :8080
```

启动后访问 `http://127.0.0.1:8080`，健康检查地址为 `http://127.0.0.1:8080/api/health`。

## 构建发布二进制

```bash
mkdir -p dist
go build -trimpath -o dist/jsonctl ./cmd/jsonctl
go build -trimpath -o dist/jsonx-webui ./cmd/webui
```

## Docker 打包

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh jsonx linux/amd64
./build_benzhi_docker.sh jsonx-arm64 linux/arm64
docker run --rm -it jsonx:latest
```

容器保留完整 Go 工具链，进入容器后可再次执行 `go build ./...` 和 `go test ./...`。
