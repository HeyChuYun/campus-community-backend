# 只保留 Windows 本地开发所需目标。
# 统一使用文件配置模式，默认读取 config/shared.local.yaml 与 config/services/*.yaml。

.PHONY: local-windows local-stop local-windows-% local-widnwo-%

# 本地环境标识。
APP_ENV ?= local

# 本地开发统一走文件配置。
CONFIG_SOURCE ?= file

# 预留 etcd 参数，便于本地 RPC 注册发现和后续切换配置中心。
ETCD_ENDPOINTS ?= 127.0.0.1:2379
CONFIG_NAMESPACE ?= /hfjj

# 统一指定配置目录。
CONFIG_DIR ?= config

# 各服务默认端口。
GATEWAY_PORT ?= 10000
PASSPORT_PORT ?= 20000
SCHOOL_PORT ?= 30000
RELATION_PORT ?= 40000
CONTENT_PORT ?= 50000
IM_PORT ?= 60000
CAPTCHA_SOLVER_PORT ?= 61000
UPDATE_SERVICE_PORT ?= 62000

export APP_ENV CONFIG_SOURCE ETCD_ENDPOINTS CONFIG_NAMESPACE CONFIG_DIR
export GATEWAY_PORT PASSPORT_PORT SCHOOL_PORT RELATION_PORT CONTENT_PORT IM_PORT CAPTCHA_SOLVER_PORT UPDATE_SERVICE_PORT

# 打开所有服务，每个服务一个 PowerShell 窗口。
local-windows:
	go run ./pkg/cmd/localdev file-windows

# 打开单个服务窗口，示例：make local-windows-relation
local-windows-%:
	go run ./pkg/cmd/localdev file-windows $*

# 兼容你刚才输入的命令拼写，示例：make local-widnwo-relation
local-widnwo-%:
	go run ./pkg/cmd/localdev file-windows $*

# 关闭 localdev 记录过的所有本地进程。
local-stop:
	go run ./pkg/cmd/localdev stop
