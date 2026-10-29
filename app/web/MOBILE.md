# OrinForum Mobile 使用指南

> 将 Web 应用打包为 iOS / Android H5 混合 App，基于 [Capacitor](https://capacitorjs.com/)。

## 目录

- [快速开始](#快速开始)
- [前置条件](#前置条件)
- [配置](#配置)
- [本地开发](#本地开发)
- [生产构建](#生产构建)
- [CI/CD 自动构建](#cicd-自动构建)
- [命令速查](#命令速查)
- [常见问题](#常见问题)

---

## 快速开始

**3 步上手**（以 iOS 为例）：

```bash
cd app/web

# 1. 初始化（首次）
cp .env.mobile.example .env.mobile   # 编辑配置
make init                            # 安装依赖
make add-ios                         # 创建 iOS 原生项目
make icon                            # 生成图标

# 2. 开发调试（HMR 热重载）
make dev                             # 一键启动 Vite + iOS 模拟器

# 3. 出包
make sync && make open-ios           # 在 Xcode 配置签名
make build-ios                       # 生成 IPA
```

**Android 同理**：把 `ios` 换成 `android`，`open-ios` 换成 `open-android`。

---

## 前置条件

### 通用

- **Node.js** 20+
- **pnpm** 10+（`npm install -g pnpm`）
- **Xcode** 15+（iOS 必需）
- **Android Studio** + JDK 17 + Gradle（Android 必需）

### 验证安装

```bash
node --version              # 应输出 v20.x 或更高
pnpm --version              # 应输出 10.x 或更高
xcodebuild -version         # iOS：应输出 Xcode 15.x
pod --version               # iOS：CocoaPods
java --version              # Android：应输出 17.x
```

### 安装 Xcode 命令行工具（如未安装）

```bash
xcode-select --install
sudo xcodebuild -license accept
```

### 安装 CocoaPods（iOS 必需）

```bash
sudo gem install cocoapods
# 或
brew install cocoapods
```

---

## 配置

### `.env.mobile`（必需）

从模板创建并编辑：

```bash
cp .env.mobile.example .env.mobile
```

关键配置项：

```bash
# 开发服务器地址（HMR 模式）
# - 设置后：原生壳加载此 URL，支持 Vite HMR
# - 留空：原生壳加载打包进 app 的静态资源（生产模式）
MOBILE_SERVER_URL=http://192.168.8.3:3000

# App 唯一标识（反向域名，发布后不可更改）
CAPACITOR_APP_ID=com.orinforum.app

# App 显示名称
CAPACITOR_APP_NAME=OrinForum

# iOS 开发团队 ID（签名用）
IOS_DEV_TEAM=
```

### 获取本机局域网 IP

```bash
make ip
```

输出示例：

```
本机局域网 IP 候选：
192.168.8.3

💡  复制到 .env.mobile：MOBILE_SERVER_URL=http://192.168.8.3:3000
```

### 查看当前配置

```bash
make status
```

---

## 本地开发

### 开发模式（HMR 热重载）

开发时原生 App 加载 Vite dev server，修改代码后自动刷新。

**方式 1：一键启动**（推荐）

```bash
make dev                        # 默认 iOS
PLATFORM=android make dev       # 切换 Android
```

**方式 2：两终端**（更稳定）

终端 A：
```bash
make dev-vite                   # 启动 Vite dev server
```

终端 B：
```bash
make dev-ios                    # 或 make dev-android
```

### 在真机调试

1. iPhone 用 USB 连接 Mac，信任开发者证书
2. Android 开启「开发者选项」→「USB 调试」
3. 运行 `make dev-ios` 或 `make dev-android`，选择真机目标

---

## 生产构建

### iOS 出包

```bash
# 1. 构建并同步 Web 资源
make sync

# 2. 打开 Xcode 配置签名
make open-ios
#    → 选择开发团队 → 勾选「自动管理签名」

# 3. 构建 IPA
make build-ios
```

产物：`ios/App/build/OrinForum.xcarchive` 和导出目录。

**手动导出 IPA**（更灵活）：

1. 在 Xcode 打开 `ios/App/App.xcworkspace`
2. Product → Archive
3. Distribute App → 选择分发方式（App Store / Ad Hoc / Development）
4. 导出 IPA

### Android 出包

```bash
# 1. 构建并同步
make sync

# 2. 生成调试 APK（无需签名）
make build-android
# 产物：android/app/build/outputs/apk/debug/app-debug.apk

# 3. 生成发布 AAB（需配置签名密钥）
make release-android
# 产物：android/app/build/outputs/bundle/release/app-release.aab
```

### 生成发布签名密钥（Android 首次）

```bash
keytool -genkey -v -keystore android/app/release.keystore \
  -alias orinforum -keyalg RSA -keysize 2048 -validity 10000
```

修改 `android/app/build.gradle` 添加 release signingConfig：

```gradle
android {
    signingConfigs {
        release {
            storeFile file('release.keystore')
            storePassword '你的密码'
            keyAlias 'orinforum'
            keyPassword '你的密码'
        }
    }
    buildTypes {
        release {
            signingConfig signingConfigs.release
        }
    }
}
```

---

## CI/CD 自动构建

通过 GitHub Actions 自动构建，详见 [`.github/workflows/MOBILE_CI_README.md`](../.github/workflows/MOBILE_CI_README.md)。

### 触发方式

**方式 1：标签推送**（推荐发版流程）

```bash
git tag v1.0.0
git push origin v1.0.0
```

→ 自动构建 release APK/AAB/IPA → 上传到 GitHub Release。

**方式 2：手动触发**

1. GitHub 仓库 → **Actions** → **Mobile Build**
2. **Run workflow** → 选择平台和构建类型
3. 点击 **Run workflow**

### 必需 Secrets

在仓库 **Settings → Secrets → Actions** 配置：

| Secret | 平台 | 说明 |
|--------|------|------|
| `ANDROID_KEYSTORE_BASE64` | Android | `base64 -i release.keystore` |
| `ANDROID_KEYSTORE_PASSWORD` | Android | 密钥库密码 |
| `ANDROID_KEY_ALIAS` | Android | 密钥别名 |
| `ANDROID_KEY_PASSWORD` | Android | 密钥密码 |
| `IOS_CERTIFICATE_P12_BASE64` | iOS | `base64 -i Certificates.p12` |
| `IOS_CERTIFICATE_PASSWORD` | iOS | p12 密码 |
| `IOS_PROVISIONING_PROFILE_BASE64` | iOS | `base64 -i profile.mobileprovision` |
| `IOS_TEAM_ID` | iOS | Apple Developer Team ID |

---

## 命令速查

```bash
make help    # 显示所有命令
```

### 初始化

| 命令 | 说明 |
|------|------|
| `make init` | 安装 Capacitor 依赖（首次使用） |
| `make add-ios` | 创建 iOS 原生项目 |
| `make add-android` | 创建 Android 原生项目 |
| `make add` | 同时创建 iOS + Android |

### 构建 + 同步

| 命令 | 说明 |
|------|------|
| `make build` | 构建 Web 生产包（`vite build`） |
| `make sync` | 构建 Web 并同步到所有原生项目 |

### 开发

| 命令 | 说明 |
|------|------|
| `make dev-vite` | 仅启动 Vite dev server |
| `make dev-ios` | iOS 模拟器运行（需另开终端跑 `dev-vite`） |
| `make dev-android` | Android 模拟器运行 |
| `make dev` | 一键启动 Vite + iOS（`PLATFORM=android` 切换） |

### IDE

| 命令 | 说明 |
|------|------|
| `make open-ios` | 在 Xcode 打开 iOS 项目 |
| `make open-android` | 在 Android Studio 打开 |

### 出包

| 命令 | 说明 |
|------|------|
| `make build-ios` | 构建 iOS IPA（需先 `open-ios` 配置签名） |
| `make build-android` | 构建 Android 调试 APK |
| `make release-android` | 构建 Android 发布 AAB |

### 维护

| 命令 | 说明 |
|------|------|
| `make icon` | 从 `pwa-512x512.png` 生成全套图标 |
| `make splash` | 生成启动屏（需提供 `resources/splash.png`） |
| `make assets` | 同时生成图标 + 启动屏 |
| `make clean` | 清理构建产物（保留原生项目） |
| `make clean-all` | 清理 + 删除原生项目（⚠️ 不可逆） |

### 工具

| 命令 | 说明 |
|------|------|
| `make status` | 显示当前配置摘要 |
| `make ip` | 打印本机局域网 IP |
| `make help` | 显示帮助 |

---

## 常见问题

### Q: `make dev-ios` 报错 "No such module" / "Command PhaseScriptExecution failed"

**解决**：首次需安装 CocoaPods：

```bash
cd ios/App && pod install && cd ../..
make dev-ios
```

### Q: 开发模式改代码后 App 不刷新

**检查**：
1. `.env.mobile` 中 `MOBILE_SERVER_URL` 是否正确（手机和电脑需在同一局域网）
2. Vite dev server 是否在跑（另开终端 `make dev-vite`）
3. 浏览器直接访问 `http://<IP>:3000` 是否能打开

### Q: `make build-ios` 失败 "No signing certificate found"

**解决**：先在 Xcode 配置签名：

```bash
make open-ios
# → 选择 Signing & Capabilities → 勾选 Automatically manage signing → 选择团队
```

### Q: Android 构建失败 "Keystore file not found"

**原因**：未配置发布签名。

**解决**：
- 调试用：`make build-android`（生成 debug APK，无需签名）
- 发布用：按「生产构建 → Android 出包」配置 release.keystore

### Q: 图标生成后不生效

**解决**：重新同步：

```bash
make icon
make sync
```

### Q: iOS runner 在 GitHub Actions 一直 pending

**原因**：`macos-latest` 对私有仓库收费，可能没有 macOS minutes。

**解决**：
- 公开仓库：免费额度
- 私有仓库：购买 macOS minutes 或注释掉 `ios` job

### Q: 如何修改 App ID 或名称？

**修改 `.env.mobile`**：

```bash
CAPACITOR_APP_ID=com.yourname.orinforum
CAPACITOR_APP_NAME=你的 App 名
```

⚠️ App ID 发布后不可更改。修改后需删除 `ios/` 和 `android/` 重新 `make add`：

```bash
make clean-all
make add
```

### Q: 如何添加原生插件（如相机、地理位置）？

```bash
pnpm add @capacitor/camera
pnpm dlx cap sync
```

如需原生权限声明（如 iOS 相机权限），在 Xcode 的 `Info.plist` 添加 `NSCameraUsageDescription`。

### Q: 如何更新 Capacitor 版本？

```bash
pnpm add -D @capacitor/core@latest @capacitor/cli@latest @capacitor/ios@latest @capacitor/android@latest
pnpm dlx cap sync
```

### Q: 如何彻底清理重建？

```bash
make clean-all     # 删除 ios/ android/
make init
make add
make icon
make sync
```

---

## 文件结构

```
app/web/
├── Makefile                     # 构建命令入口
├── capacitor.config.ts          # Capacitor 配置（读取 .env.mobile）
├── .env.mobile.example          # 配置模板（提交到 git）
├── .env.mobile                  # 本地配置（gitignored）
├── public/
│   ├── pwa-192x192.png          # PWA + 原生图标源
│   └── pwa-512x512.png
├── ios/                         # iOS 原生项目（gitignored）
├── android/                     # Android 原生项目（gitignored）
└── MOBILE.md                    # 本指南

.github/workflows/
├── mobile-build.yml             # GitHub Actions workflow
└── MOBILE_CI_README.md          # CI 配置指南
```

---

## 参考链接

- [Capacitor 官方文档](https://capacitorjs.com/docs)
- [Capacitor iOS Configuration](https://capacitorjs.com/docs/ios/configuration)
- [Capacitor Android Configuration](https://capacitorjs.com/docs/android/configuration)
- [GitHub Actions macOS Runners](https://docs.github.com/en/actions/using-github-hosted-runners/about-github-hosted-runners)

---

**最后更新**：2026-06-17
