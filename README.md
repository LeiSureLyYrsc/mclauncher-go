# mclauncher-go

「忘却的旋律」Minecraft 启动器 5.31（2014）的 Go 重写。界面是 Qt Quick / QML，不是网页。

原程序是加了 UPX 的易语言 DirectUI 程序，没有源码。主窗口按当时的 720×450 布局摆：右下角四个按钮、选中的版本、打开目录。背景和标题栏按钮用的是从原 exe 里拆出来的图，放在 `assets/skins`，没有重画。分析写在 [ANALYSIS.md](ANALYSIS.md)。

它和原作者、Mojang、Microsoft 都没有关系。

## 能做什么

- 扫描 `.minecraft/versions`，启动选中的版本
- 把 1.6 之前的 `.minecraft/bin/minecraft.jar` 导入成一个版本
- 离线进入（原版默认）
- Yggdrasil 账号：自己填验证服务器根地址
- 微软账号设备码登录
- 从 `https://piston-meta.mojang.com/mc/game/version_manifest_v2.json` 安装正式版或快照，带库和资源，用 SHA1 跳过已下完的文件
- 内存自动（500–1300 MB，和 5.31 的说明一致）或手动
- 版本隔离、调试模式（`mclauncher.sh` / `mclauncher.bat` + `输出日志.txt`）

密码只留在登录那一次的内存里，不会写进配置。令牌会写进配置文件，权限是 `0600`。

## 构建

命令行工具不需要 Qt，用来检查目录和打印启动命令：

```bash
go test ./internal/config ./internal/auth ./internal/java ./internal/launch ./internal/version ./internal/download
go build -o bin/mclauncher-cli ./cmd/mclauncher-cli
./bin/mclauncher-cli scan --dir ~/.minecraft
./bin/mclauncher-cli command --dir ~/.minecraft --version 1.20.1 --name Melody
```

窗口需要 Qt 6.4+（Core、Gui、Qml、Quick）、C 编译器和 pkg-config。绑定是 [miqt v0.15.0](https://github.com/mappu/miqt)。QML 在 `assets/qml/Main.qml`，运行时从可执行文件旁边或当前目录找 `assets/`。

```bash
# Debian / Ubuntu
sudo apt install gcc pkg-config qt6-base-dev qt6-declarative-dev
go build -o bin/mclauncher ./cmd/mclauncher
```

Windows 上安装 Qt 6，并让编译器找得到 Qt。界面字体优先微软雅黑。

## 配置

第一次运行会在用户配置目录建立 `mclauncher/mclauncher.cfg`（Linux 一般是 `~/.config/mclauncher/mclauncher.cfg`）。游戏目录默认是 `~/.minecraft`，可以在设置里改成启动器旁边的目录，贴近原版「旁边放一个 .minecraft」的习惯。

第三方皮肤站如果要求 authlib-injector，把 jar 路径填在设置里。原版 5.31 没有这一项，这是为了让 Yggdrasil 根地址在新版本上真的能进服务器。

## 布局

```text
cmd/mclauncher/        启动 QML 窗口
cmd/mclauncher-cli/    无界面
assets/qml/            Qt Quick 界面
assets/skins/          从 2014 年 exe 拆出的背景和按钮，不是新画的
internal/auth/         离线 UUID、Yggdrasil、微软设备码
internal/launch/       版本 JSON → java 命令行
internal/download/     官方清单安装
internal/version/      扫描、导入、重命名
internal/config/       mclauncher.cfg
internal/ui/           QML 状态桥
```
