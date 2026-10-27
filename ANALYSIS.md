# 原版 mclauncher 5.31 分析

附件是一个 614,912 字节的 Windows 可执行文件，不是源码包。解包后可以确定它就是国内早期启动器「忘却的旋律」的最终公开版。

## 文件身份

| 项 | 值 |
| --- | --- |
| 文件 | `mclauncher.exe` |
| 版本 | `5.31.0.0`（产品名 MCLauncher / Minecraft 启动器） |
| 时间戳 | 2014-09-08 08:57:52 UTC |
| 版权字符串 | 忘却的旋律 版权所有 |
| 格式 | PE32，子系统 Windows GUI，基址 `0x400000` |
| 壳 | UPX（节区 `UPX0` / `UPX1` / `.rsrc`）。UPX 4.2.4 可还原到约 1.1 MB |
| 运行库 | 易语言（`krnln` 风格的运行时错误串：「内存不足」「调用 DLL 命令后发现堆栈错误」等） |
| 界面 | `Ex_DirectUI`，皮肤图来自 Adobe Fireworks CS6 / Macromedia Fireworks 8（2012–2013） |
| 字体 | 界面写死「微软雅黑」，编辑框「宋体」 |
| 联网 | WinINet：`InternetOpenA`，User-Agent 伪装成 MSIE 6.0 |

导入表在加壳状态下只留了每个 DLL 的一个跳板函数，这是 UPX 的正常现象，不是功能列表。真实依赖从解包字符串看，包括 kernel32、user32、gdiplus、wininet、shell32、advapi32（注册表里的 `jarfile` 打开命令）、MSXML HTTP。

## 主界面在做什么

主按钮就是这五个：

- 启动游戏
- 更新游戏
- 下载中心
- 设置
- 打开目录

角标文案是 `by 忘却的旋律`，链接 `http://www.mcsky.net`。旁边是版本列表（「选中的版本」）。第一次启动会弹出「请完成以下设置」，并在 `mclauncher.cfg` 里记下「第一次运行」。

设置窗口（字符串里的标题是「游戏设置窗口」）包含：

- 游戏名字
- 最大内存，单位 MB，默认看到的数字是 1024
- 内存设置方式：自动 / 手动
- java 路径，浏览，或「自动下载」
- 调试模式
- 查找 java / 保存设置
- 正版登录测试 / 删除正版信息
- 启动器皮肤（`bgskin\*.jpg`，「随机皮肤」）
- 游戏目录：保存到 `.minecraft`，或保存到 `.minecraft\versions\版本名称\`（「只在 1.6 以上的版本有效」，对应后来的版本隔离）
- 删除选中游戏、重命名

自动内存的原文是：按系统剩余内存，最大 1300M，最小 500M。更大的内存要手动设。

## 启动命令

它同时认识两代目录。

**1.6 之前**（`.minecraft\bin\minecraft.jar`）：

```text
javaw -Xincgc -Xmx{MB}M -cp "...\minecraft.jar;...\lwjgl.jar;...\lwjgl_util.jar;...\jinput.jar" -Djava.library.path="...\natives" net.minecraft.client.Minecraft <名字> <session>
```

发现旧目录时提示「当前目录发现旧版本游戏,开始导入」，把 `bin` 收进 `versions\<名字>\`，并写一个标记文件「已经导入.删除本文件重新导入」。

**1.6 及以后**走 `versions\<id>\<id>.json`：

- 读取 `libraries[].name` / `url`，缺省库地址 `https://libraries.minecraft.net/`
- 读取 `mainClass`、`minecraftArguments`
- 替换 `${auth_player_name}` `${auth_uuid}` `${auth_access_token}` `${auth_session}` `${user_type}` `${user_properties}` `${version_name}` `${game_directory}` `${assets_root}` `${game_assets}` `${assets_index_name}`
- 额外加上 Forge 时代很常见的  
  `-Dfml.ignoreInvalidMinecraftCertificates=true -Dfml.ignorePatchDiscrepancies=true`
- 调试模式不用 `javaw.exe`，改用 `java.exe`，写出 `mclauncher.bat`，输出到 `输出日志.txt`

正版登录打到当时的 Yggdrasil：

```text
POST https://authserver.mojang.com/authenticate
{"agent":{"name":"Minecraft","version":1},"username":"邮箱","password":"密码","requestUser":true}
```

令牌字段是 `accessToken`、`selectedProfile.id`、`selectedProfile.name`。客户端档案写在 `.minecraft\launcher_profiles.json`，profile 名叫 `MClauncher`。登录文件放在 `%APPDATA%\mclauncher\login`。

Java 查找顺序能从字符串还原：已保存路径、注册表 `SOFTWARE\Classes\jarfile\shell\open\command\`、文件名 `javaw.exe|java.exe`。找不到就提示手动指定，或按 `IsWow64Process` 下载 64/32 位安装包到 `\mctemp\java.temp.exe`。

## 下载

版本清单当时是：

```text
http://s3.amazonaws.com/Minecraft.Download/versions/versions.json
http://s3.amazonaws.com/Minecraft.Download/versions/<id>/
http://s3.amazonaws.com/Minecraft.Download/indexes/<id>.json
http://resources.download.minecraft.net/<hh>/<hash>
```

下载中心的分类是：简单整合、官方原版、Forge、Forge+OptiFine。列表来自

- `http://updatemc.mcsky.net/launchernew/gameupdate.ini`
- `http://updatemc.mcsky.net/launchernew/forgeupdate.ini`
- `http://updatemc.mcsky.net/launchernew/foupdate.ini`

资源慢的时候引导到 `http://pan.mcsky.net/`。启动器自己的更新看 `checkversion` 里的 `latestversion`，下载 `update.exe`，用 `ping` 延迟后改名替换自己。背景和 natives 还有 `newmcl.googlecode.com` 的 svn 备份。公告图来自 `http://launcher.sinaapp.com/m/?post=2`。

这些 mcsky / sinaapp / googlecode 地址在 2026 年都不应再当作可用服务。

## 和这次重写的对应关系

| 原版 | mclauncher-go |
| --- | --- |
| 易语言 + Ex_DirectUI + UPX | Go，界面用 Qt Quick（QML），经 [miqt](https://github.com/mappu/miqt) 调用 |
| 离线名字 | 保留，离线 UUID 仍是 `OfflinePlayer:` 的 UUIDv3 |
| Mojang 邮箱密码 | 保留为可配置的 Yggdrasil 根地址（LittleSkin、ely.by 这类还活着；`authserver.mojang.com` 已关闭） |
| 无微软登录 | 补了设备码登录。2022 年后正版只能走这条 |
| mcsky 整合包 ini | 不内置已死的镜像。下载中心改拉官方 `version_manifest_v2.json` |
| 自动下 Java | 不再从第三方下 JRE，只查找本机 `java` / `JAVA_HOME` |
| `mclauncher.cfg` | 用户配置目录里的同名 JSON 文件，权限 `0600`，不写密码 |
| 导入 `bin/`、版本隔离、调试脚本 | 保留 |
