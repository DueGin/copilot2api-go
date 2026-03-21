# CI/CD 配置文档

## 概述

本项目使用 **GitHub Actions** 实现自动化构建与部署。当推送版本标签后，流水线会自动完成编译、Docker 镜像构建、推送至镜像仓库，并通过 SSH 部署到生产服务器。

## 触发条件

```yaml
on:
  push:
    tags:
      - 'v*'
```

**唯一触发方式：推送以 `v` 开头的 Git 标签。**

```bash
# 示例：创建并推送标签
git tag v1.0.0
git push origin v1.0.0
```

以下操作 **不会** 触发流水线：
- 推送代码到任何分支（main、develop 等）
- 创建 Pull Request
- 合并 Pull Request
- 手动在 GitHub Actions 页面点击运行

## 流水线架构

```
推送 v* 标签
    │
    ├─── build-release（并行）
    │       │
    │       ├── 构建前端（Bun）
    │       ├── 交叉编译 5 个平台的 Go 二进制
    │       └── 发布到 GitHub Releases
    │
    └─── docker（并行）
            │
            ├── 构建多架构 Docker 镜像（amd64 + arm64）
            └── 推送至 GHCR（GitHub Container Registry）
                    │
                    │ ✅ 镜像推送完成
                    ▼
                deploy（串行）
                    │
                    ├── 生成 docker-compose.yml（替换镜像地址）
                    ├── SCP 传输到服务器
                    └── SSH 登录服务器 → pull 镜像 → 重启容器
```

### 作业依赖关系

| 作业 | 依赖 | 说明 |
|------|------|------|
| `build-release` | 无 | 独立运行，编译二进制并发布 Release |
| `docker` | 无 | 独立运行，构建并推送 Docker 镜像 |
| `deploy` | `docker` | 等待镜像推送完成后，SSH 部署到服务器 |

`build-release` 和 `docker` 并行执行，`deploy` 必须等 `docker` 成功后才运行。

## 三个作业详解

### 1. build-release — 编译二进制

在 GitHub Runner 上编译多平台二进制文件，并上传到 GitHub Releases。

**构建平台：**

| 操作系统 | 架构 | 产物格式 |
|----------|------|----------|
| Linux | amd64 | `.tar.gz` |
| Linux | arm64 | `.tar.gz` |
| Windows | amd64 | `.zip` |
| macOS | amd64 | `.tar.gz` |
| macOS | arm64 | `.tar.gz` |

**构建步骤：**
1. Bun 安装依赖并构建前端
2. Go 交叉编译各平台二进制（`CGO_ENABLED=0`，静态链接）
3. 打包前端产物 + 二进制到压缩包
4. 上传到 GitHub Releases（自动生成 Release Notes）

### 2. docker — 构建并推送镜像

使用三阶段 Dockerfile 构建最小化生产镜像，推送到 GHCR。

**Dockerfile 三阶段：**

```
阶段 1: oven/bun:1-alpine       → 构建前端
阶段 2: golang:1.25-alpine      → 编译 Go 二进制
阶段 3: alpine:3.21             → 最终运行时镜像（仅包含二进制 + 前端产物）
```

**镜像标签规则：**

推送 `v1.2.3` 标签时，生成以下镜像标签：

| 标签 | 示例 |
|------|------|
| `{{version}}` | `ghcr.io/<owner>/<repo>:1.2.3` |
| `{{major}}.{{minor}}` | `ghcr.io/<owner>/<repo>:1.2` |
| `latest` | `ghcr.io/<owner>/<repo>:latest` |

**多架构支持：**
- 构建 `linux/amd64` 和 `linux/arm64` 两个架构
- 使用 QEMU + Docker Buildx 实现交叉构建
- 推送的是多架构 manifest，服务器 `docker pull` 时自动匹配本机架构

### 3. deploy — 部署到服务器

等待 `docker` 作业成功后，通过 SSH 将服务部署到生产服务器。

**部署步骤：**
1. 用 `sed` 将 `docker-compose.prod.yml` 中的 `${DOCKER_IMAGE}` 替换为实际镜像地址
2. 通过 SCP 将生成的 `docker-compose.yml` 传输到服务器的 `~/copilot2api-go/` 目录
3. SSH 登录服务器，执行：
   - 登录 GHCR 镜像仓库
   - `docker compose pull` 拉取最新镜像
   - `docker compose up -d --remove-orphans` 重启服务
   - `docker image prune -f` 清理旧镜像

**服务端口映射：**

| 服务 | 容器端口 | 主机端口 |
|------|----------|----------|
| Web 控制台 | 3000 | 3180 |
| 代理 API | 4141 | 4141 |

## GitHub 配置指南

### 第一步：创建 Environment

1. 进入仓库页面 → **Settings** → **Environments**
2. 点击 **New environment**
3. 名称填写 `production`，点击 **Configure environment**

> Environment 可以配置保护规则，例如需要审批人确认后才能部署。如不需要审批，直接创建即可。

### 第二步：配置 Repository Secrets

进入仓库 → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**

需要配置以下 Secrets：

| Secret 名称 | 说明 | 示例 |
|---|---|---|
| `DEPLOY_HOST` | 服务器 IP 地址或域名 | `123.45.67.89` |
| `DEPLOY_USER` | SSH 登录用户名 | `root` |
| `DEPLOY_SSH_KEY` | SSH 私钥（完整内容，包含首尾行） | `-----BEGIN OPENSSH PRIVATE KEY-----`... |
| `DEPLOY_PORT` | SSH 端口号 | `22` |
| `GHCR_USER` | GitHub 用户名（用于服务器登录 GHCR） | `your-github-username` |
| `GHCR_TOKEN` | GitHub Personal Access Token（需要 `read:packages` 权限） | `ghp_xxxxxxxxxxxx` |

### 第三步：生成 SSH 密钥对

如果还没有用于部署的 SSH 密钥，执行以下命令：

```bash
# 1. 在本地生成密钥对
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/deploy_key

# 2. 将公钥添加到服务器的 authorized_keys
ssh-copy-id -i ~/.ssh/deploy_key.pub <用户名>@<服务器IP>

# 3. 查看私钥内容，复制到 GitHub Secrets 的 DEPLOY_SSH_KEY
cat ~/.ssh/deploy_key
```

### 第四步：生成 GHCR Token

1. 进入 GitHub → 头像 → **Settings** → **Developer settings**
2. **Personal access tokens** → **Tokens (classic)** → **Generate new token**
3. 勾选 `read:packages` 权限
4. 生成后复制 token，填入 GitHub Secrets 的 `GHCR_TOKEN`

> 这个 token 用于服务器从 GHCR 拉取私有镜像。如果仓库和镜像是公开的，则可以省略 GHCR 登录步骤。

## 服务器环境要求

部署服务器需要安装：

| 软件 | 最低版本 | 检查命令 |
|------|----------|----------|
| Docker | 20.10+ | `docker --version` |
| Docker Compose V2 | 2.0+ | `docker compose version` |

> 注意：需要使用 `docker compose`（V2 插件形式），而非旧版 `docker-compose`。

## 相关文件

| 文件 | 用途 |
|------|------|
| `.github/workflows/release.yml` | CI/CD 流水线定义 |
| `Dockerfile` | 三阶段 Docker 镜像构建 |
| `docker-compose.yml` | 本地开发环境 |
| `docker-compose.prod.yml` | 生产部署模板（CI 中替换镜像地址后使用） |

## 常用操作

### 发布新版本

```bash
git tag v1.0.0
git push origin v1.0.0
```

### 查看流水线状态

在仓库的 **Actions** 标签页查看运行状态和日志。

### 手动回滚

如果新版本有问题，SSH 到服务器手动回滚：

```bash
cd ~/copilot2api-go

# 修改 docker-compose.yml 中的镜像标签为指定版本
# 例如将 :latest 改为 :1.0.0
vim docker-compose.yml

docker compose pull
docker compose up -d
```

### 查看服务器上的容器状态

```bash
ssh <用户名>@<服务器IP>
cd ~/copilot2api-go
docker compose ps
docker compose logs -f
```
