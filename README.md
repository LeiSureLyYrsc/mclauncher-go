# mclauncher-go

「忘却的旋律」Minecraft 启动器 5.31（2014）的 Go 重写。界面是 Qt Quick / QML，不是网页。

原程序是加了 UPX 的易语言 DirectUI 程序，没有源码。主窗口按当时的 720×450 布局摆：右下角四个按钮、选中的版本、打开目录。背景和标题栏按钮用的是从原 exe 里拆出来的图，放在 `assets/skins`，没有重画。分析写在 [ANALYSIS.md](ANALYSIS.md)。

它和原作者、Mojang、Microsoft 都没有关系。

## 能做什么

- 扫描 `.minecraft/versions`，启动选中的版本
- 把 1.6 之前的 `.minecraft/bin/minecraft.jar` 导入成一个版本
- 不填账号就是离线。名字空着会用 Player，不需要先做一遍设置
- 外置登录走 Authlib Injector。验证服务器、账号、密码在设置里填，jar 可以自己选，也可以从 BMCLAPI 下最新的
- 微软登录还没接
- 版本清单、客户端、库和资源可选官方源或 [BMCLAPI](https://bmclapi2.bangbang93.com)。正式版、快照、远古版都能安装、更新和启动，用 SHA1 跳过已经下完的文件。BMCLAPI 会把文件跳到志愿节点，节点连不上就换一个，再失败则改走官方
- Java 路径留空时，会在 PATH、JAVA_HOME、常见安装目录和 `.minecraft/runtime` 里嗅探，并按版本 json 的 `javaVersion` 选一个够新的
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

Windows 上安装 Qt 6，并让编译器找得到 Qt。界面字体优先微软雅黑。自己编译出来的窗口仍然依赖系统里的 Qt。给别人用的是下面的单文件。

## 自动构建

[.github/workflows/build.yml](.github/workflows/build.yml) 在推到 `main`、打开 pull request、或打 `v*` 标签时跑测试并打出**一个**可执行文件：

- `mclauncher-linux-amd64`
- `mclauncher-windows-amd64.exe`

Qt 库、QML 插件和皮肤都在这个文件里。第一次打开会解到用户缓存（Linux `~/.cache/mclauncher`，Windows `%LOCALAPPDATA%\mclauncher`），不需要另外装 Qt，旁边也不用放 DLL。Linux 这个文件是在 Ubuntu 24.04 上编的，系统 glibc 太旧会打不开。

标签会把这两个文件挂到 GitHub Release。每次构建的 Artifacts 里也能下。

## 配置

第一次运行不会强制打开设置。游戏目录默认是 `~/.minecraft`。离线不写登录配置。外置登录的令牌会写进配置文件，权限是 `0600`，密码不保存。Authlib Injector 默认放在同一配置目录。

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
