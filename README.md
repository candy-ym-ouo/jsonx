# jsonx

`jsonx` 是一个从零实现的 Go JSON 解析、序列化与校验项目。核心编解码不依赖 `encoding/json`，包含：

- 手写词法器、递归下降解析器和保留对象键顺序的值树
- 行号、列号、字节偏移、JSONPath、错误码和上下文片段
- Go 结构体反射编解码、严格模式、错误聚合和循环引用检测
- 轻量 JSON Schema 校验（类型、对象、数组、范围、模式、组合等）
- `jsonctl` 命令行工具与离线 Web Playground

## 验证

```bash
go build ./...
go vet ./...
go test ./... -count=1
go test -race ./...
```

## 运行

```bash
go run ./cmd/jsonctl format example.json -i 2
go run ./cmd/webui -addr :8080
```

浏览器打开 `http://127.0.0.1:8080`。生产打包可执行：

```bash
mkdir -p dist
go build -trimpath -o dist/jsonctl ./cmd/jsonctl
go build -trimpath -o dist/jsonx-webui ./cmd/webui
```

项目要求的统计口径为 `gofmt` 后的物理行（含注释和空行），排除所有 `*_test.go` 文件。
