# Android Shell

这是 Todo 的 Android 原生壳工程。目标不是把现有 Go 服务改写成新的移动端，而是：

- 用 Android 原生 WebView 直接打开现有 Todo 网页
- 保留现有网页前端和后端
- 后续可继续接原生能力，例如短信读取

## 当前结构

- `web/`
  预留的本地桥接壳页面，后续如果要恢复离线壳或加本地配置页，可以继续使用
- `android/`
  Capacitor 生成的 Android 工程

## 当前行为

- App 启动后直接打开远端 Todo 网页
- 实际服务地址从本地私有配置读取
- 不再显示本地包裹面板或 iframe 容器
- 后续短信能力仍然保留 Android 原生插件扩展位

首次使用前，请先从示例文件复制一份本地配置：

```bash
cd android-shell
cp capacitor.config.example.json capacitor.config.json
```

然后把 `capacitor.config.json` 里的 `server.url` 和 `allowNavigation` 改成你自己的服务地址。这个文件已经加入 `.gitignore`，不会提交到仓库。

## 图标与启动图

安卓图标和启动图都从仓库根目录的 `logo.png` 生成。

如果你后面替换了 logo，可以重新执行：

```bash
cd android-shell
npm run assets:android
npm run sync
npm run build:debug
```

## 首次准备

在 `android-shell/` 目录下执行：

```bash
npm install
npx cap add android
```

如果已经生成过 Android 工程，后续同步：

```bash
npx cap sync android
```

## 打开 Android 工程

```bash
npx cap open android
```

## 直接打调试包

当前工程已经验证过可以在 macOS 上用 JDK 21 直接打出 Android 调试 APK。

```bash
cd android-shell
export JAVA_HOME=$(/usr/libexec/java_home -v 21)
npm run assets:android
npm run sync
npm run build:debug
```

生成结果：

```text
android/app/build/outputs/apk/debug/app-debug.apk
```

如果只是继续改 Android 配置或原生代码，通常执行：

```bash
npm run sync
```

再重新打包即可。

## 运行前说明

- 需要本机安装 Android Studio 和 Android SDK
- 当前已确认本机 JDK 21 可正常构建；JDK 24/25 下 Gradle 可能报 class version 错误
- 打包前请确保本地 `capacitor.config.json` 已配置正确的服务地址
- AndroidManifest 已开启 `usesCleartextTraffic=true`，所以后面如果要切回局域网 `http://IP:8080` 也能测
- 当前这版为了避免登录 Cookie 在 iframe 下失效，已经改成顶层 WebView 直开站点

## 后续短信接口

当前已经预留两层接口：

- Android 原生插件：`SmsBridgePlugin`

后面如果要接短信，可以直接把桥接能力注入当前网页，不需要推翻现有架构。
