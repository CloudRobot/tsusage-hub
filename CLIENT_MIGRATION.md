# tsdaily / tsusage 跨平台移植指南 (macOS & Windows)

本指南介绍如何将本地的 `tsdaily` 脚本及命令行包装器 `tsusage` 迁移配置到其他 **macOS** 或 **Windows** 设备上，并接入 `tsusage-hub` 聚合看板。

相关文件已归档在当前仓库的 `client/` 目录下：
- `client/tsdaily.mjs`
- `client/tsusage`
- `client/tsusage.cmd`
- `client/README.md`

---

## 一、核心构成与依赖要求

### 1. 核心文件说明
- `tsdaily.mjs`：核心执行脚本（纯 JS，单文件，基于 `tokscale` + Cindy SQLite + Trae SQLite 读取用量）。
  - 目标部署位置：`~/.local/share/tsdaily/tsdaily.mjs`（Win: `%USERPROFILE%\.local\share\tsdaily\tsdaily.mjs`）
- `tsusage`：macOS / Linux / Git Bash 包装脚本（使用 `$HOME` 变量，天然跨平台）。
  - 目标部署位置：`~/.local/bin/tsusage`
- `tsusage.cmd`：Windows CMD / PowerShell 批处理包装脚本（使用 `%USERPROFILE%`，天然跨平台）。
  - 目标部署位置：`%USERPROFILE%\.local\bin\tsusage.cmd`

### 2. 运行环境要求
- **Node.js >= 22.5.0**（推荐 Node 22 LTS 或 Node 24，因脚本引入了 Node 原生内置模块 `node:sqlite`）。
- **tokscale**（可选但强烈推荐全局安装）：
  ```bash
  npm install -g tokscale
  ```
  *(若未安装，脚本会自动 fallback 走 `npx tokscale@4.16.0`)*

---

## 二、移植到 macOS

### 1. 创建目录并分发文件
在目标 Mac 终端执行：
```bash
# 1. 创建目标目录
mkdir -p ~/.local/share/tsdaily ~/.local/bin

# 2. 复制文件到对应目录
# - 将 tsdaily.mjs 复制到 ~/.local/share/tsdaily/tsdaily.mjs
# - 将 tsusage     复制到 ~/.local/bin/tsusage

# 3. 赋予执行权限
chmod +x ~/.local/bin/tsusage
```

### 2. 将 bin 目录加入 PATH
编辑当前用户的终端配置文件（例如 `~/.zshrc` 或 `~/.bash_profile`）：
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

### 3. 配置 Hub 上报环境变量（按需）
若需要自动汇总到 central Hub，在 `~/.zshrc` 末尾添加：
```bash
export TSUSAGE_HUB="http://<UNRAID_IP>:3888"
export TSUSAGE_MACHINE="MacBook-Pro"  # 自定义设备标识，不填默认取主机名
# export TSUSAGE_AUTH_TOKEN="your_token" # 若 Hub 启用了 Token 校验
```

### 4. 配置后台静默上报（可选）
使用 crontab 每 30 分钟静默推送一次用量：
```bash
crontab -e
# 添加以下行：
*/30 * * * * $HOME/.local/bin/tsusage --sync >/dev/null 2>&1
```

---

## 三、移植到 Windows

### 1. 创建目录并分发文件
在目标 Windows 的 PowerShell 终端中执行：
```powershell
# 1. 创建目标目录
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.local\share\tsdaily", "$env:USERPROFILE\.local\bin"

# 2. 复制文件到对应目录：
# - 将 tsdaily.mjs 复制到 $env:USERPROFILE\.local\share\tsdaily\tsdaily.mjs
# - 将 tsusage     复制到 $env:USERPROFILE\.local\bin\tsusage (供 Git Bash / WSL 使用)
# - 将 tsusage.cmd 复制到 $env:USERPROFILE\.local\bin\tsusage.cmd (供 CMD / PowerShell 使用)
```

### 2. 将 bin 目录加入用户 PATH
在 PowerShell 中执行以下命令（设置后重新打开终端窗口生效）：
```powershell
$binPath = "$env:USERPROFILE\.local\bin"
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -split ';' -notcontains $binPath) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$binPath", 'User')
}
```

### 3. 配置 Hub 上报环境变量（按需）
在 PowerShell 中永久写入用户环境变量：
```powershell
[System.Environment]::SetEnvironmentVariable('TSUSAGE_HUB', 'http://<UNRAID_IP>:3888', 'User')
[System.Environment]::SetEnvironmentVariable('TSUSAGE_MACHINE', 'Desktop-Win', 'User')
# [System.Environment]::SetEnvironmentVariable('TSUSAGE_AUTH_TOKEN', 'your_token', 'User')
```

### 4. 配置后台静默上报（可选）
使用 Windows「任务计划程序」或直接在 PowerShell 中注册定时任务（每 30 分钟静默上报一次）：
```powershell
$action = New-ScheduledTaskAction -Execute "$env:USERPROFILE\.local\bin\tsusage.cmd" -Argument "--sync"
$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 30) -RepetitionDuration ([TimeSpan]::MaxValue)
Register-ScheduledTask -TaskName "tsusage-sync" -Action $action -Trigger $trigger -Description "Sync AI Token Usage to Hub"
```

---

## 四、验证与检查

在终端运行：
```bash
tsusage
```
- 若显示今日/近期各模型与客户端的消耗表格，并在配置了 Hub 时提示 `[tsusage] Successfully synced usage to http://...`，即表示移植与多端同步完全就绪。
- 支持的常用参数：
  - `tsusage`：展示今日消耗表
  - `tsusage week`：展示本周消耗表
  - `tsusage month`：展示本月消耗表
  - `tsusage 7d`：展示近 7 天消耗
  - `tsusage --sync`：仅在后台静默上报数据到 Hub，不打印表格
