.PHONY: all build windows linux clean run check cover lint docker install uninstall help

BIN_FILE = app
HARBOR = harbor.fgj.com/pub
TAG = latest
CODE_REPORT = golangci-lint-report.xml
WINDOWS_ENV= CGO_ENABLED=0 GOOS=windows GOARCH=amd64
LINUX_ENV = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
COVER_REPORT = cover.out

# @符号表示抑制命令的打印，使得命令执行时不会在终端显示命令本身

#all: 通常用来作为默认目标，执行一系列其他目标
all: clean docker install

#go build -gcflags="-m" -o ${BIN_FILE}
build:	
	@go build -ldflags="-s -w" -installsuffix cgo -o ${BIN_FILE}

windows:
	@$(WINDOWS_ENV) go build -ldflags="-s -w" -installsuffix cgo -o ${BIN_FILE}

linux:
	@$(LINUX_ENV) go build -ldflags="-s -w" -installsuffix cgo -o ${BIN_FILE}

#go clean命令清理Go编译生成的中间文件
clean:
	@go clean
	@rm -f ${COVER_REPORT} ${CODE_REPORT}

#运行项目中的所有测试用例  go test -run "TestOne|TestTwo" ./...
test:
	@go test ./...

#go fmt ./...：格式化项目中所有Go源文件，确保代码风格统一
#go vet ./...：使用go vet对源码进行静态分析，检测潜在的错误或可疑构造
check:
	@go fmt ./...
	@go vet ./...

#go tool cover -html="xxx.out" 使用go tool cover将xxx.out转换为HTML格式的覆盖率报告，并在浏览器中打开
#go tool cover -func="xxx.out" 终端输出测试报告
cover:
	@go test ./... -coverprofile="${COVER_REPORT}" -timeout=30s
	@go tool cover -html="${COVER_REPORT}"

run:
	@./${BIN_FILE}

# 增加超时时间, 在 github action 中载入所有检查模块比较慢, 本地是因为安装了这些模块
# 检查当前目录所有.go文件，会递归查找当前目录及子目录
lint:
	@golangci-lint run -c .golangci.yaml -v --timeout=30s

docker:
	@docker build -t ${HARBOR}/${BIN_FILE}:${TAG} .
	@docker push ${HARBOR}/${BIN_FILE}:${TAG}

install:
	@cd deploy/helm/app && ./install.sh

uninstall:
	@cd deploy/helm/app && ./uninstall.sh

help:
	@echo "make: 一键部署应用"
	@echo "make build: 编译go代码生成二进制文件"
	@echo "make windows: 编译go代码生成windows二进制文件"
	@echo "make linux: 编译go代码生成linux二进制文件"
	@echo "make clean: 清理中间目标文件"
	@echo "make test: 执行测试case"
	@echo "make check: 格式化go代码"
	@echo "make cover: 检查测试覆盖率"
	@echo "make run: 直接运行程序"
	@echo "make lint: 执行代码检查"
	@echo "make docker: 构建docker镜像并上传仓库"
	@echo "make install: 部署应用"
	@echo "make uninstall: 卸载应用"
