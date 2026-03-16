# Synology 部署说明

这套说明专门针对:

- 开发机: macOS, Apple Silicon (`arm64`)
- 群晖: x86_64 / Intel 8100T (`amd64`)

之前容易出问题的根因是:

- 你在 mac 上如果直接 `docker build`，默认很容易产出 `linux/arm64`
- 群晖那边是 `linux/amd64`
- 架构不一致时，镜像能导入但跑不起来，或者直接提示 `exec format error`

这套导出脚本会强制生成 `linux/amd64` 镜像，不会再把 `arm64` 包带到群晖。

## 目录说明

导出后你会拿到:

- `todo-app-synology-amd64.tar`
  - 这是群晖可以直接导入的应用镜像
- `docker-compose.with-db.yml`
  - 应用 + PostgreSQL 一起在群晖上跑
- `docker-compose.external-db.yml`
  - 只跑应用，数据库用外部 PostgreSQL
- `.env.example`
  - 环境变量模板

## 一. 在 mac 上导出 amd64 镜像

先确认 Docker Desktop 已经启动。

在项目根目录运行:

```bash
chmod +x scripts/export_synology_amd64.sh
./scripts/export_synology_amd64.sh
```

导出成功后，会得到两个重要文件:

- `dist/synology-amd64/todo-app-synology-amd64.tar`
- `dist/todo-synology-amd64-bundle.tar.gz`

推荐你直接把 `dist/todo-synology-amd64-bundle.tar.gz` 上传到群晖。

## 二. 上传到群晖

把下面这个文件上传到群晖任意目录，例如:

- `/volume1/docker/todo/todo-synology-amd64-bundle.tar.gz`

然后 SSH 到群晖:

```bash
ssh <你的群晖用户名>@<群晖IP>
```

解压:

```bash
mkdir -p /volume1/docker/todo
cd /volume1/docker/todo
tar -xzf todo-synology-amd64-bundle.tar.gz
ls -la
```

解压后你应该能看到:

- `todo-app-synology-amd64.tar`
- `docker-compose.with-db.yml`
- `docker-compose.external-db.yml`
- `.env.example`
- `README.md`

## 三. 选择部署模式

### 方案 A: 群晖里自己带 PostgreSQL

适合你想把 Todo 和数据库都放在群晖里。

复制环境文件:

```bash
cp .env.example .env
```

至少改这几个值:

- `POSTGRES_PASSWORD`
- `DATABASE_URL`
  - 要和上面的密码保持一致
- `SESSION_SECURE_COOKIE`
  - 如果你后面会挂 HTTPS 反代，改成 `true`
- `QUOTES_DATABASE_URL`
  - 如果没有 quotes 库就留空

启动:

```bash
docker load -i todo-app-synology-amd64.tar
docker compose --env-file .env -f docker-compose.with-db.yml up -d
```

查看状态:

```bash
docker compose -f docker-compose.with-db.yml ps
docker compose -f docker-compose.with-db.yml logs -f app
```

访问:

- `http://<群晖IP>:8080`

### 方案 B: 群晖只跑应用，数据库继续用外部 PostgreSQL

适合你已经有现成 PostgreSQL，不想在群晖里再起一个。

复制环境文件:

```bash
cp .env.example .env
```

重点修改:

- `DATABASE_URL`
  - 例如:
    `postgres://username:password@192.168.1.10:5432/todo?sslmode=disable`
- `QUOTES_DATABASE_URL`
  - 有就填，没有就留空
- `SESSION_SECURE_COOKIE`
  - 如果走 HTTPS 反代，改成 `true`

这时 `POSTGRES_DB / POSTGRES_USER / POSTGRES_PASSWORD` 可以不再使用。

启动:

```bash
docker load -i todo-app-synology-amd64.tar
docker compose --env-file .env -f docker-compose.external-db.yml up -d
```

查看状态:

```bash
docker compose -f docker-compose.external-db.yml ps
docker compose -f docker-compose.external-db.yml logs -f app
```

访问:

- `http://<群晖IP>:8080`

## 四. 如果你想用群晖图形界面

如果你用的是 Synology Container Manager，也可以走 UI:

1. `映像` -> `新增` -> `从文件新增`
2. 选择 `todo-app-synology-amd64.tar`
3. 导入完成后，去 `项目`
4. 新建项目，选择你解压出来的 compose 文件
5. 把 `.env.example` 复制成 `.env` 并填好
6. 用对应的 compose 文件启动

如果图形界面在 `.env` 处理上不稳定，优先用 SSH 命令行方式，最稳。

## 五. 常见问题

### 1. 导入后启动报 `exec format error`

说明你导入的不是 `linux/amd64` 包，或者导错文件了。

正确文件必须是:

- `todo-app-synology-amd64.tar`

并且它必须由下面这个命令生成:

```bash
./scripts/export_synology_amd64.sh
```

### 2. 页面打不开

先看容器有没有起来:

```bash
docker ps
```

再看日志:

```bash
docker logs todo-app
```

### 3. 数据库连不上

如果你用的是内置 PostgreSQL，先看数据库容器:

```bash
docker logs todo-db
```

如果你用的是外部 PostgreSQL，先检查 `.env` 里的 `DATABASE_URL`。

### 4. 首个账号怎么处理

首个注册账号会自动成为 admin。

后续账号注册后，需要 admin 审批才可登录。
