# 官方 Go 镜像保留完整工具链，便于容器内继续编译和测试。
FROM golang:1.22

WORKDIR /app

# 项目仅使用 Go 标准库，无需下载外部模块依赖。
COPY . .

# 预编译项目并保留构建缓存。
RUN go build ./...

CMD ["bash"]
