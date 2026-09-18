# tsusage-hub

极轻量、单人多设备的 AI Token 用量聚合中心与实时监控大屏。
专为搭配 `tsdaily` / `tsusage` 设计，常驻内存仅约 10MB，Docker 镜像仅约 15MB。

---

## 一、部署流程（GitHub Actions + Unraid Docker Compose）

### 1. 推送代码到 GitHub
在 GitHub 上创建一个新的公开（Public）或私有（Private）仓库，例如命名为 `tsusage-hub`，然后在本地执行：

```bash
cd F:\opencode\tsusage-hub
git remote set-url origin git@CloudRobot:CloudRobot/tsusage-hub.git
git push -u origin main
```

推送后，进入 GitHub 仓库页面点击 **Actions**，工作流将自动编译 multi-arch 镜像并推送到 GitHub Container Registry (GHCR)。

### 2. 设置 GHCR 镜像公开（免去在 Unraid 配置登录）
1. 在 GitHub 个人主页或仓库右侧，找到 **Packages** -> 进入 `tsusage-hub`；
2. 点击 **Package settings**；
3. 滚动到页面底部 **Danger Zone** -> 点击 **Change package visibility**，设置为 **Public**。
*(设置为 Public 后，Unraid 无需配置 GitHub Token 即可直接拉取)*

---

## 二、Unraid 部署（Docker Compose）

在 Unraid 的 **Docker Compose Manager** 中新建一个项目，或在 Unraid 终端直接创建：

```yaml
services:
  tsusage-hub:
    image: ghcr.io/cloudrobot/tsusage-hub:latest
    container_name: tsusage-hub
    restart: unless-stopped
    ports:
      - "3888:3888"
    volumes:
      - /mnt/user/appdata/tsusage-hub/data:/data
    environment:
      - PORT=3888
      - DB_PATH=/data/usage.db
      - TZ=Asia/Shanghai
      # - AUTH_TOKEN=your_token_if_needed  # 可选：防止内网非授权上报
```

点击 **Compose Up** 启动。
启动后即可通过浏览器访问：`http://<UNRAID_IP>:3888` 实时查看大屏。

---

## 三、客户端配置（各台电脑）

### 1. 配置环境变量
在你的 Windows / Mac 电脑上添加环境变量：
```powershell
# Windows PowerShell（临时生效测试）
$env:TSUSAGE_HUB="http://192.168.1.100:3888"

# Windows 永久生效（用户环境变量）
[System.Environment]::SetEnvironmentVariable('TSUSAGE_HUB', 'http://192.168.1.100:3888', 'User')

# 可选：自定义机器显示名称（不配置则默认取电脑主机名）
[System.Environment]::SetEnvironmentVariable('TSUSAGE_MACHINE', 'Desktop-Win', 'User')
```

### 2. 正常使用
每次在终端运行：
```bash
tsusage
```
会自动将本机今天截至目前的用量推送到 Unraid，并立即拉取并打印跨所有机器合并后的统一汇总表！

### 3. 后台静默自动同步
运行：
```bash
tsusage --sync
```
该指令仅在后台静默上报最新数据，不在终端输出大表，适合放入 Windows 任务计划程序或 macOS Crontab 每 30 分钟定时自动执行。
