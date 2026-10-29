# OrinForum Mobile Build — Makefile
# 基于 Capacitor，支持 iOS / Android H5 混合打包
#
# 快速开始：
#   1. cp .env.mobile.example .env.mobile   # 按需修改
#   2. make init                            # 首次安装依赖
#   3. make add-ios / make add-android      # 初始化原生项目（一次性）
#   4. make sync && make open-ios           # 构建并在 Xcode 打开

# ===================================================================
#  环境变量读取
# ===================================================================
ifneq (,$(wildcard .env.mobile))
  include .env.mobile
  export
endif

MOBILE_SERVER_URL    ?=
CAPACITOR_APP_ID     ?= com.orinforum.app
CAPACITOR_APP_NAME   ?= OrinForum
IOS_DEV_TEAM         ?=

# ===================================================================
#  路径与命令
# ===================================================================
PWD_DIR     := $(shell pwd)
OUTPUT_DIR  := .output/public
CAP          = pnpm dlx cap
VITE         = pnpm exec vite

# ===================================================================
#  辅助函数
# ===================================================================
define require_platforms
	@test -d ios -o -d android || { \
		echo "❌ 未检测到原生项目，请先运行 make add-ios 或 make add-android"; \
		exit 1; \
	}
endef

define require_output
	@test -d "$(OUTPUT_DIR)" || { \
		echo "❌ 未检测到 Web 构建产物 $(OUTPUT_DIR)，请先运行 make build"; \
		exit 1; \
	}
endef

# ===================================================================
#  初始化（首次使用）
# ===================================================================
.PHONY: init add-ios add-android add

init: ## 安装 Capacitor 核心与 CLI 依赖（首次使用）
	pnpm add -D @capacitor/core @capacitor/cli dotenv
	@echo "✅  Capacitor 已安装，下一步运行 make add-ios 或 make add-android"

add: add-ios add-android ## 同时添加 iOS 和 Android 平台

add-ios: ## 初始化 iOS 原生项目（需先 make init）
	@test ! -d ios || { echo "ℹ️  ios/ 已存在，跳过"; exit 0; }
	$(CAP) add ios
	@echo "✅  iOS 项目已创建在 ios/"

add-android: ## 初始化 Android 原生项目（需先 make init）
	@test ! -d android || { echo "ℹ️  android/ 已存在，跳过"; exit 0; }
	$(CAP) add android
	@echo "✅  Android 项目已创建在 android/"

# ===================================================================
#  构建 + 同步
# ===================================================================
.PHONY: build sync

build: ## 构建 Web 生产包（vite build）
	$(VITE) build

sync: build ## 构建 Web 并同步到所有原生项目
	$(call require_platforms)
	$(CAP) sync
	@echo "✅  Web 资源已同步到原生项目"

# ===================================================================
#  开发模式（HMR 热重载）
# ===================================================================
.PHONY: dev dev-vite dev-ios dev-android

dev-vite: ## 仅启动 Vite dev server（配合 dev-ios/dev-android 使用）
	$(VITE) --host 0.0.0.0

dev-ios: ## iOS 模拟器运行（需 .env.mobile 设置 MOBILE_SERVER_URL + 另一个终端跑 dev-vite）
	$(call require_platforms)
	$(CAP) run ios --external
	@echo "ℹ️  若未自动启动，请另开终端运行 make dev-vite"

dev-android: ## Android 模拟器运行（需 .env.mobile 设置 MOBILE_SERVER_URL + 另一个终端跑 dev-vite）
	$(call require_platforms)
	$(CAP) run android --external
	@echo "ℹ️  若未自动启动，请另开终端运行 make dev-vite"

dev: ## 一键启动：后台 Vite + 前台 iOS/Android（通过 PLATFORM=android 切换）
	$(call require_platforms)
	@if [ -z "$(MOBILE_SERVER_URL)" ]; then \
		echo "❌ 请在 .env.mobile 设置 MOBILE_SERVER_URL（如 http://192.168.x.x:3000）"; \
		exit 1; \
	fi
	@echo "🚀  启动 Vite dev server（后台）..."
	$(VITE) --host 0.0.0.0 > /tmp/orinforum-vite.log 2>&1 & \
		echo $$! > /tmp/orinforum-vite.pid
	@sleep 4
	@echo "📱  启动 $(if $(filter android,$(PLATFORM)),Android,iOS) 模拟器..."
	$(CAP) run $(if $(filter android,$(PLATFORM)),android,ios) --external; \
		STATUS=$$?; \
		if [ -f /tmp/orinforum-vite.pid ]; then \
			kill `cat /tmp/orinforum-vite.pid` 2>/dev/null || true; \
			rm -f /tmp/orinforum-vite.pid; \
		fi; \
		exit $$STATUS

# ===================================================================
#  打开原生 IDE（签名、权限、原生代码调试）
# ===================================================================
.PHONY: open-ios open-android

open-ios: ## 在 Xcode 打开 iOS 项目（配置签名、证书、Capabilities）
	$(call require_platforms)
	@test -d ios || { echo "❌ 未找到 ios/，请先 make add-ios"; exit 1; }
	$(CAP) open ios

open-android: ## 在 Android Studio 打开（配置 Gradle、签名密钥）
	$(call require_platforms)
	@test -d android || { echo "❌ 未找到 android/，请先 make add-android"; exit 1; }
	$(CAP) open android

# ===================================================================
#  生产构建（出包）
# ===================================================================
.PHONY: build-ios build-android release-ios release-android

build-ios: sync ## 构建并生成 iOS .ipa（需先 make open-ios 配置签名）
	@test -d ios || { echo "❌ 未找到 ios/"; exit 1; }
	@echo "📦  开始 iOS 构建（首次构建需在 Xcode 完成签名配置）..."
	@if [ -n "$(IOS_DEV_TEAM)" ]; then \
		cd ios/App && xcodebuild -workspace App.xcworkspace \
			-scheme App -configuration Release \
			-archivePath build/OrinForum.xcarchive archive \
			DEVELOPMENT_TEAM=$(IOS_DEV_TEAM) \
			CODE_SIGN_STYLE=Automatic \
			| tail -20; \
	else \
		echo "ℹ️  未设置 IOS_DEV_TEAM，请在 Xcode 手动配置签名后执行："; \
		echo "    cd ios/App && xcodebuild -workspace App.xcworkspace -scheme App -configuration Release archive"; \
	fi

build-android: sync ## 构建并生成 Android .apk（调试包）
	@test -d android || { echo "❌ 未找到 android/"; exit 1; }
	cd android && ./gradlew assembleDebug
	@echo "✅  APK 位于 android/app/build/outputs/apk/debug/"

release-android: sync ## 构建并生成 Android .aab（Google Play 发布包）
	@test -d android || { echo "❌ 未找到 android/"; exit 1; }
	cd android && ./gradlew bundleRelease
	@echo "✅  AAB 位于 android/app/build/outputs/bundle/release/"

# ===================================================================
#  维护工具
# ===================================================================
.PHONY: icon splash assets clean status help

icon: ## 从 public/pwa-512x512.png 生成全套原生图标（需 @capacitor/assets）
	pnpm dlx @capacitor/assets generate --icon --ios --android \
		--icon-source public/pwa-512x512.png
	@echo "✅  图标已生成，请重新运行 make sync"

splash: ## 生成启动屏（需提供 resources/splash.png 1024x1024）
	pnpm dlx @capacitor/assets generate --splash --ios --android
	@echo "✅  启动屏已生成，请重新运行 make sync"

assets: icon splash ## 同时生成图标 + 启动屏

clean: ## 清理所有构建产物（保留 ios/ android/ 项目配置）
	rm -rf $(OUTPUT_DIR) dist .vinxi
	@test -d ios    && rm -rf ios/App/build    ios/App/Pods    ios/App/App.xcworkspace || true
	@test -d android && rm -rf android/.gradle android/app/build android/build || true
	@echo "✅  已清理"

clean-all: clean ## 清理 + 删除原生项目（⚠️  不可逆，签名/原生代码会丢失）
	@read -p "⚠️  确认删除 ios/ 和 android/？签名配置、原生代码均会丢失 (y/N) " c; \
	[ "$$c" = "y" ] || { echo "已取消"; exit 0; }
	rm -rf ios android
	@echo "✅  原生项目已删除"

status: ## 显示当前配置摘要
	@echo "📋  OrinForum Mobile 配置"
	@echo "─────────────────────────────────"
	@echo "App ID:        $(CAPACITOR_APP_ID)"
	@echo "App Name:      $(CAPACITOR_APP_NAME)"
	@if [ -n "$(MOBILE_SERVER_URL)" ]; then \
		echo "Server URL:    $(MOBILE_SERVER_URL)  ← 开发模式（HMR）"; \
	else \
		echo "Server URL:    (未设置 → 生产模式，打包静态资源)"; \
	fi
	@echo "iOS Team:      $(if $(IOS_DEV_TEAM),$(IOS_DEV_TEAM),(未设置))"
	@echo "─────────────────────────────────"
	@echo "Web 构建:      $(if $(wildcard $(OUTPUT_DIR)/index.html),✅ 存在,❌ 未构建)"
	@echo "iOS 项目:      $(if $(wildcard ios/App/App.xcodeproj),✅ 已初始化,❌ 未初始化)"
	@echo "Android 项目:  $(if $(wildcard android/build.gradle),✅ 已初始化,❌ 未初始化)"

ip: ## 打印本机局域网 IP（用于 MOBILE_SERVER_URL）
	@echo "本机局域网 IP 候选："
	@if command -v ipconfig >/dev/null 2>&1; then \
		ipconfig getifaddr en0 2>/dev/null || true; \
		ipconfig getifaddr en1 2>/dev/null || true; \
	else \
		ip -4 addr show 2>/dev/null | grep -oP 'inet \K[\d.]+' | grep -v '127.0.0.1' || echo "(无法检测)"; \
	fi
	@echo ""
	@echo "💡  复制到 .env.mobile：MOBILE_SERVER_URL=http://<IP>:3000"

# ===================================================================
#  帮助
# ===================================================================
.PHONY: help
help: ## 显示本帮助
	@echo "OrinForum Mobile Build"
	@echo "======================"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "用法: make \033[36m<target>\033[0m\n\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
