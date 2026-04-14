# Trae Switch

**Trae Switch** 是一个专为 Trae IDE 设计的工具，通过 DNS 劫持 + 本地反向代理，让 Trae IDE 支持第三方大模型服务商 API（如阿里百炼 Coding Plan、Kimi Coding Plan 等）。

当前仓库是面向 macOS 的版本，目标是让你在 macOS 上直接把 Trae 的 OpenAI 请求转发到自定义第三方服务商。

## 🚀 功能特点

- **多服务商支持**：可添加、编辑、删除多个服务商配置
- **CA 证书管理**：生成并安装本地 CA 证书，用于 HTTPS 拦截
- **实时状态监控**：显示代理运行状态和当前激活的服务商
- **不需要输入key**：通过在trae中配置key，在本工具不需要输入任何apikey

## 📋 支持的服务商

- ✅ 支持 /v1 服务商
- ✅ 其他支持 OpenAI 协议的第三方api服务商

## 🔧 技术架构

### 技术栈
- **后端**：Go (Wails 框架)
- **前端**：Svelte + Tailwind CSS
- **网络**：HTTPS 代理服务器
- **系统**：macOS 系统集成（Hosts 管理、证书安装、443 端口转发）

### 核心模块
1. **代理服务器**：监听 443 端口，处理 OpenAI API 请求
2. **配置管理**：读写 `config.json` 配置文件
3. **Hosts 管理**：自动设置和恢复 Hosts 配置
4. **证书管理**：生成和安装自签名 CA 证书
5. **前端界面**：现代化的用户交互界面

## 📦 macOS 拉源码后怎么跑

### 1. 环境要求

请先确保本机具备下面这些依赖：

- macOS 11.0 及以上
- Xcode Command Line Tools
- Go 1.24 或更高版本
- Node.js 18+ 和 npm

如果你还没装 Xcode Command Line Tools，可以先执行：

```bash
xcode-select --install
```

### 2. 拉代码

```bash
git clone https://github.com/z1737029714/trae-switch-mac.git
cd trae-switch-mac
```

### 3. 构建 macOS App

```bash
./scripts/build-macos.sh
```

这个脚本会自动完成下面几件事：

- 如果 `frontend/node_modules` 不存在，会自动执行 `npm install`
- 执行前端构建
- 编译 Go 后端
- 生成 `build/Trae Switch.app`
- 如果存在 `build/appicon.png`，会自动打包成应用图标
- 如果本机 `127.0.0.1:7890` 有可用代理，会自动用于依赖下载；没有就直接走默认网络，不需要手动设置环境变量

### 4. 启动 App

构建完成后直接打开：

```bash
open "build/Trae Switch.app"
```

也可以在 Finder 里双击 `build/Trae Switch.app`。

### 5. 第一次启动建议按这个顺序操作

1. 打开应用后，先点击「一键授权」
2. 如果弹出系统密码框，输入一次即可
3. 再安装 CA 证书，并在系统钥匙串里设为信任
4. 添加你的第三方 provider
5. 点击「启动」开启代理

说明：

- 「一键授权」主要用于 Hosts 和 443 端口转发，正常情况下只需要做一次
- 后续日常开关代理，不应该每次都重复要求输入系统密码
- 安装或信任 CA 证书时，macOS 仍可能弹出一次系统级确认，这是系统行为

### 6. 复现构建 + 启动

```bash
cd trae-switch-mac
./scripts/build-macos.sh
open "build/Trae Switch.app"
```

## 🛠️ 使用方法

### 1. 添加服务商配置
1. 点击「+ 添加」按钮
2. 填写服务商信息：
   - **名称**：服务商显示名称（如 "阿里百炼"）
   - **API 地址**：OpenAI 协议的 API 地址（如 `https://coding.dashscope.aliyuncs.com/v1`）
   - **模型列表**：添加可用的模型 ID（如 `qwen3.5-plus`、`kimi-k2.5` 等）
3. 点击「保存」

### 2. 启动代理
1. 确保系统配置中的「Hosts 配置」和「CA 证书」都已启用
2. 点击右上角的「启动」按钮
3. 代理启动成功后，状态栏会显示「运行中」

### 3. 在 Trae IDE 中使用
1. 打开 Trae IDE
2. 进入模型设置
3. 选择 OpenAI 服务商
4. 输入对应第三方服务商的真实 API Key（如 `sk-xxx`）
5. 手动输入你想要使用的模型名称
6. 关闭 auto mode 并选择刚添加的模型
7. 开始使用！

## ⚙️ 配置文件

如果你是通过源码构建并直接打开 `build/Trae Switch.app`，配置文件默认位于：

```bash
build/Trae Switch.app/Contents/MacOS/config.json
```

格式如下：

```json
{
  "providers": [
    {
      "name": "服务商名称",
      "openai_base": "https://api.example.com/v1",
      "models": ["model-1", "model-2"]
    }
  ],
  "active_provider": 0
}
```

- `name`：服务商显示名称
- `openai_base`：OpenAI 协议的 API 地址
- `models`：模型 ID 列表
- `active_provider`：当前激活的服务商索引

## 📝 使用说明

1. 添加服务商配置（API 地址和模型列表）并点击「启动」
2. 在 Trae IDE 添加自定义模型，服务商选择 OpenAI 服务商
3. 模型手动输入你想要使用的模型并且输入对应 API Key（如 sk-xxx）
4. 关闭 auto mode 并且选择刚添加的模型

## 🔍 常见问题

### Q: 启动失败怎么办？
**A:** 请检查：
- 是否使用的是 macOS 11.0 及以上
- `xcode-select --install` 是否已执行
- `go version` 和 `npm -v` 是否可用
- 443 端口是否被占用
- Hosts 配置是否成功
- CA 证书是否安装
- 第一次打开未签名应用时，是否已在 Finder 里右键应用并选择「打开」

### Q: 模型不显示怎么办？
**A:** 请确保：
- 已在服务商配置中添加了模型
- 已选择了正确的服务商
- 代理已成功启动

### Q: API Key 如何获取？
**A:** API Key 需要从对应服务商的官方网站获取

### Q: 支持哪些模型？
**A:** 支持所有支持openai接口协议服务商提供的模型，只要在配置中添加对应的模型 ID 即可。

## 🛡️ 安全性

- **本地运行**：所有数据处理都在本地进行，不会上传任何数据
- **自签名证书**：仅用于本地 HTTPS 拦截，不会影响其他应用
- **Hosts 修改**：仅修改 `api.openai.com` 的解析，不影响其他域名
- **不存储key**：通过在trae中配置key，在本工具不需要输入任何key


## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)。

---
## Star History

<a href="https://www.star-history.com/?repos=mtfly%2Ftrae-switch&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/image?repos=mtfly/trae-switch&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/image?repos=mtfly/trae-switch&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/image?repos=mtfly/trae-switch&type=date&legend=top-left" />
 </picture>
</a>
