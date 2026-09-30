# tsusage-hub

极轻量、单人多设备的 AI Token 用量聚合中心与实时监控大屏。
专为搭配 `tsdaily` / `tsusage` 设计，常驻内存仅约 10MB，Docker 镜像仅约 15MB。

### 核心特性
- **多维度统计**：支持 **按日**、**按周**（自然周）、**按月**（自然月）三个维度自由切换聚合。
- **算力活跃热力图 (Activity Heatmap)**：
  - 支持 **近1年**、**近半年**、**近3个月** 时间范围自由切换。
  - 支持 **Tokens** / **USD 费用** 双指标切换与 5 档自然绿阶色彩渲染。
  - 具备左侧星期标尺与顶部月份对齐，展示年度总用量、出勤活跃天数与比例、历史单日最高峰值、活跃日均消耗。
  - **双向联动下钻**：点击任意方块一键跳转至当日大屏明细，高亮光环实时指示当前大屏所在日期。
- **厂商用量统计与深度下钻 (By Vendor)**：
  - 智能识别 OpenAI、Anthropic、Google、DeepSeek、Moonshot (Kimi)、阿里通义 (Qwen)、智谱 GLM、腾讯混元、字节豆包、MiniMax、Meta / Llama、Mistral、百度文心、阶跃星辰、零一万物、NVIDIA 等主流厂商与开源模型。
  - 汇总各厂商总 Tokens、总预估费用、占大盘费用百分比条。
  - **点开查看各个模型细节**：
    - 展示各厂商内部输入/输出比例、Context Caching 缓存命中量与缓存节省率。
    - 展开各个模型的完整细节表格：**Input / Output / CacheRead (高亮命中) / CacheWrite / 总 Tokens / 预估费用 / 厂商内占比迷你进度条 / 贡献设备 / 别名合并提示**。
    - 厂商底部提供小计行（Subtotal）核对。
  - 支持 **一键全部展开/折叠**、**多维排序**（按费用/Tokens/模型数）与 **即时搜索过滤**。
- **便捷时间导航**：
  - 点击日期卡片任意区域即可秒级唤出原生日期/月份选择面板。
  - 支持 `◀` / `▶` 快捷前后步进（前一天/上一周/上个月、后一天/下一周/下个月），无需反复打开日历即可连续翻看。
  - 提供 `今天` / `本周` / `本月` 一键快速回到当前。
- **多设备聚合与拆分**：支持跨设备合并去重与按设备拆分明细（By Machine）。
- **实时监控大屏**：开箱即用的大屏深色看板，支持模型粒度 Input / Output / 缓存命中 / 费用全方位监控。

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
