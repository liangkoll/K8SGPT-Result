# 使用 golang 官方镜像作为基础镜像
FROM af.hikvision.com.cn/docker-mvd/golang:latest

# 将本地编译好的可执行文件复制到镜像中
COPY k8sgpt_result /app/k8sgpt_result

# 设置工作目录
WORKDIR /app

# 暴露端口
EXPOSE 8080

# 运行可执行文件
CMD ["/app/k8sgpt_result"]