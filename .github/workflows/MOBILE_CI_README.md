# Mobile Build CI/CD 配置说明

本 workflow 通过 GitHub Actions 自动构建 Android APK/AAB 和 iOS IPA。

## 触发条件

| 触发类型 | 行为 |
|---------|------|
| **推送 `v*` 标签**（如 `v1.0.0`） | 自动构建 release 版本，上传到 GitHub Release |
| **手动触发**（workflow_dispatch） | 可选平台（all/android/ios）和构建类型（release/debug） |

## 必需 Secrets

在 GitHub 仓库 **Settings → Secrets and variables → Actions** 中配置。

### Android

| Secret | 说明 | 生成方式 |
|--------|------|----------|
| `ANDROID_KEYSTORE_BASE64` | 发布签名密钥库（base64） | `base64 -i release.keystore` |
| `ANDROID_KEYSTORE_PASSWORD` | 密钥库密码 | — |
| `ANDROID_KEY_ALIAS` | 密钥别名 | — |
| `ANDROID_KEY_PASSWORD` | 密钥密码 | — |

**生成 keystore**（首次）：
```bash
keytool -genkey -v -keystore release.keystore \
  -alias orinforum -keyalg RSA -keysize 2048 -validity 10000
```

**修改 `android/app/build.gradle`** 让 CI 能识别签名配置：
```gradle
android {
    signingConfigs {
        release {
            if (System.getenv("ORIN_KEYSTORE_FILE") != null) {
                storeFile file(System.getenv("ORIN_KEYSTORE_FILE"))
                storePassword System.getenv("ORIN_KEYSTORE_PASSWORD")
                keyAlias System.getenv("ORIN_KEY_ALIAS")
                keyPassword System.getenv("ORIN_KEY_PASSWORD")
            }
        }
    }
    buildTypes {
        release {
            signingConfig signingConfigs.release
            minifyEnabled false
            proguardFiles getDefaultProguardFile('proguard-android.txt'), 'proguard-rules.pro'
        }
    }
}
```

> ⚠️ 修改后需提交 `android/app/build.gradle`（虽然 `android/` 默认 gitignore，但签名配置必须入库）。
> 建议在 `android/app/build.gradle` 上 `git add -f` 强制追踪。

### iOS（macOS runner，GitHub 付费）

| Secret | 说明 | 生成方式 |
|--------|------|----------|
| `IOS_CERTIFICATE_P12_BASE64` | 发布证书 p12（base64） | `base64 -i Certificates.p12` |
| `IOS_CERTIFICATE_PASSWORD` | p12 密码 | — |
| `IOS_PROVISIONING_PROFILE_BASE64` | Provisioning Profile（base64） | `base64 -i profile.mobileprovision` |
| `IOS_TEAM_ID` | Apple Developer Team ID | 从 Apple Developer 账户查看 |

**获取证书**：
1. 登录 [Apple Developer](https://developer.apple.com/account/)
2. Certificates → 创建 Distribution 证书 → 下载 .cer
3. 钥匙串访问 → 右键证书 → 导出 → 保存为 .p12（设置密码）
4. `base64 -i Certificates.p12 | pbcopy`

**获取 Provisioning Profile**：
1. Profiles → 创建 App Store / Ad Hoc / Development Profile
2. 下载 → `base64 -i xxx.mobileprovision | pbcopy`

### 可选 Secrets

| Secret | 用途 |
|--------|------|
| `IOS_APP_STORE_CONNECT_API_KEY` | TestFlight 上传（API Key JSON） |
| `IOS_APP_STORE_CONNECT_API_KEY_ID` | API Key ID |
| `IOS_APP_STORE_CONNECT_ISSUER_ID` | Issuer ID |
| `CAPACITOR_APP_ID` | 覆盖默认 `com.orinforum.app` |
| `CAPACITOR_APP_NAME` | 覆盖显示名 `OrinForum` |

## 使用方式

### 方式 1：标签触发（推荐发版流程）

```bash
git tag v1.0.0
git push origin v1.0.0
```

→ 自动构建 release APK/AAB/IPA，上传到 GitHub Release。

### 方式 2：手动触发（调试用）

1. 进入 GitHub 仓库 → **Actions** → **Mobile Build**
2. 点击 **Run workflow**
3. 选择平台和构建类型
4. 点击 **Run workflow**

### 方式 3：本地构建（不依赖 CI）

```bash
cd app/web
make init && make add
make build-android   # 生成 APK
make build-ios       # 生成 IPA（需先 make open-ios 配置签名）
```

## 输出

每次构建完成后，artifacts 保留 30 天：

- `android-release` / `android-debug`：APK 和 AAB
- `ios-release` / `ios-debug`：IPA

标签推送时自动创建 GitHub Release 并附带安装包。

## 常见问题

### Q: iOS job 一直不跑？

GitHub Actions 的 `macos-latest` runner 对私有仓库**收费**。确认：
- 公开仓库：免费额度
- 私有仓库：需购买 macOS minutes

可临时在 workflow 中注释 `ios` job 以节省成本。

### Q: Android 构建失败 "Keystore file not found"？

未配置 `ANDROID_KEYSTORE_BASE64` secret 时，CI 会 fallback 到 debug 签名。如需 release 签名，请按上文配置 4 个 Android secrets。

### Q: iOS 导出失败 "No signing certificate found"？

确认：
1. `IOS_CERTIFICATE_P12_BASE64` 已配置且 base64 正确（无换行）
2. `IOS_PROVISIONING_PROFILE_BASE64` 已配置
3. 证书类型为 **Distribution**（用于 release）或 **Development**（用于 debug）

### Q: 如何切换 App ID？

在 GitHub Secrets 添加 `CAPACITOR_APP_ID`（如 `com.yourname.orinforum`），CI 会自动覆盖 `capacitor.config.ts` 默认值。

### Q: 如何禁用 iOS job？

编辑 `.github/workflows/mobile-build.yml`，注释或移除 `ios` job。

## 维护

- Workflow 使用 Capacitor CLI 在 CI 中动态 `add` 平台，确保 `ios/`、`android/` 不被本地 git 污染
- 每次 CI 运行都是干净环境，无需担心本地残留
- 修改 `app/web/capacitor.config.ts` 后 CI 自动生效
