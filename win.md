1. **安装 Claude CLI**
   以管理员身份打开 PowerShell，运行以下命令（任选其一）：

   - **Windows 包管理器**：`winget install Anthropic.ClaudeCode`

2. **检查并创建 PowerShell 配置文件**
   在 PowerShell 中运行 `Test-Path $PROFILE`。如果返回 `False`，说明配置文件不存在，运行 `New-Item -Path $PROFILE -ItemType File -Force` 创建一个。

3. **编辑配置文件**

   powershell

   ```
   otepad $PROFILE
   ```

   

   方法一（推荐）：

   把以下函数复制到打开的记事本并保存：

   powershell

   ```
   function claude-magic {
       $env:ANTHROPIC_BASE_URL="http://192.168.112.253:8080"
       $env:ANTHROPIC_AUTH_TOKEN="sk-magic-<你的API-Key>"
       $env:ANTHROPIC_MODEL="deepseek-v4-pro[1m]"
       $env:ANTHROPIC_DEFAULT_OPUS_MODEL="deepseek-v4-pro[1m]"
       $env:ANTHROPIC_DEFAULT_SONNET_MODEL="deepseek-v4-pro[1m]"
       $env:ANTHROPIC_DEFAULT_HAIKU_MODEL="deepseek-v4-flash"
       $env:CLAUDE_CODE_SUBAGENT_MODEL="deepseek-v4-flash"
       $env:CLAUDE_CODE_EFFORT_LEVEL="max"
       claude --model "deepseek-v4-pro[1m]" @args
   }
   ```

   

4. **重新加载配置**
   重启 PowerShell 或运行 `. $PROFILE`，之后在任意终端输入 `claude-magic` 即可启动。



方法二：

C盘User文件夹下面有一个.claude文件夹，里面创建一个settings.json文件添加
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://192.168.112.253:8080",
    "ANTHROPIC_AUTH_TOKEN": "",
    "ANTHROPIC_MODEL": "deepseek-v4-pro[1m]",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "deepseek-v4-pro[1m]",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "deepseek-v4-pro[1m]",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "deepseek-v4-flash",
    "CLAUDE_CODE_SUBAGENT_MODEL": "deepseek-v4-flash",
    "CLAUDE_CODE_EFFORT_LEVEL": "max"
  },
  "permissions": {
    "allow": [
      "Bash(pip install *)",
      "Bash(python main.py --algo reinforce --episodes 50)"
    ]
  },
  "disableLoginPrompt": true,
  "hasCompletedOnboarding": true
}

终端直接运行claude命令，就可以了