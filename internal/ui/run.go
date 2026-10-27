// Package ui is the Qt Quick window. The pictures under assets/skins are the
// ones carved out of the 2014 launcher; QML only places them.
package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
	"github.com/mappu/miqt/qt6/qml"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/auth"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/config"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/download"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/java"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/launch"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/version"
)

// App is the QML-facing state. QML reads and writes keys on the property map;
// the key "cmd" is a one-shot verb.
type App struct {
	cfg      config.Config
	password string

	bridge *qml.QQmlPropertyMap
	parent *qt.QWidget

	mu       sync.Mutex
	writing  bool
	versions []version.Installed
	remote   download.Manifest
	cancel   context.CancelFunc
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	app := &App{cfg: cfg, parent: qt.NewQWidget2()}
	app.bridge = qml.NewQQmlPropertyMap()
	app.bridge.OnValueChanged(app.onChanged)
	app.publish()

	engine := qml.NewQQmlApplicationEngine()
	engine.RootContext().SetContextProperty("app", app.bridge.QObject)
	qmlPath := findAsset("qml/Main.qml")
	engine.Load(qt.QUrl_FromLocalFile(qmlPath))
	if len(engine.RootObjects()) == 0 {
		return fmt.Errorf("QML 没有加载: %s", qmlPath)
	}
	if cfg.FirstRun {
		app.setStr("panel", "setup")
	}
	go app.loadRemote()
	qt.QApplication_Exec()
	return nil
}

func (a *App) publish() {
	c := a.cfg
	a.setStr("playerName", c.PlayerName)
	a.setInt("memoryMB", c.MaxMemoryMB)
	a.setBool("memoryAuto", c.MemoryAuto)
	a.setStr("javaPath", c.JavaPath)
	a.setBool("debug", c.Debug)
	a.setBool("isolate", c.IsolateGame)
	a.setStr("gameDir", c.GameDir)
	a.setStr("skin", c.Skin)
	a.setStr("skinUrl", fileURL(findAsset(skinFile(c.Skin))))
	a.setStr("assetsUrl", fileURL(findDir("skins")))
	a.setStr("email", c.Email)
	a.setStr("yggdrasil", c.YggdrasilRoot)
	a.setStr("authMode", c.AuthMode)
	a.setBool("firstRun", c.FirstRun)
	a.setStr("status", "就绪")
	a.setInt("progress", 0)
	a.setBool("busy", false)
	a.setStr("panel", "")
	a.setStr("log", "")
	a.setStr("remoteFilter", "release")
	a.setInt("remoteIndex", 0)
	a.setStr("cmd", "")
	a.refreshLocal()
	a.refreshRemote()
}

func (a *App) onChanged(key string, value *qt.QVariant) {
	if a.writing || value == nil {
		return
	}
	switch key {
	case "cmd":
		cmd := strings.TrimSpace(value.ToString())
		if cmd == "" {
			return
		}
		a.setStr("cmd", "")
		a.dispatch(cmd)
	case "playerName":
		a.cfg.PlayerName = value.ToString()
	case "memoryMB":
		if n := value.ToInt(); n >= 256 {
			a.cfg.MaxMemoryMB = n
		}
	case "memoryAuto":
		a.cfg.MemoryAuto = value.ToBool()
	case "javaPath":
		a.cfg.JavaPath = value.ToString()
	case "debug":
		a.cfg.Debug = value.ToBool()
	case "isolate":
		a.cfg.IsolateGame = value.ToBool()
	case "gameDir":
		a.cfg.GameDir = value.ToString()
	case "email":
		a.cfg.Email = value.ToString()
	case "yggdrasil":
		a.cfg.YggdrasilRoot = value.ToString()
	case "authMode":
		a.cfg.AuthMode = value.ToString()
	case "password":
		a.password = value.ToString()
	case "skin":
		a.cfg.Skin = value.ToString()
		a.setStr("skinUrl", fileURL(findAsset(skinFile(a.cfg.Skin))))
	case "versionIndex":
		a.selectVersion(value.ToInt())
	case "remoteFilter":
		a.refreshRemote()
	}
}

func (a *App) dispatch(cmd string) {
	switch cmd {
	case "launch":
		a.onLaunch()
	case "update":
		a.onUpdate()
	case "openDir":
		a.onOpenDir()
	case "save":
		a.onSave()
	case "findJava":
		a.onFindJava()
	case "browseJava":
		a.onBrowseJava()
	case "browseDir":
		a.onBrowseDir()
	case "login":
		a.onLogin()
	case "clearLogin":
		a.onClearLogin()
	case "importLegacy":
		a.onImport()
	case "removeVersion":
		a.onRemove()
	case "rename":
		a.onRename()
	case "install":
		a.onInstall()
	case "reloadRemote":
		go a.loadRemote()
	default:
		if strings.HasPrefix(cmd, "rename:") {
			a.applyRename(strings.TrimPrefix(cmd, "rename:"))
		}
	}
}

func (a *App) onLaunch() {
	_ = config.Save(a.cfg)
	inst, ok := a.current()
	if !ok {
		a.fail("没有选择游戏,请选择或点击更新按钮更新游戏")
		return
	}
	sess, err := a.session()
	if err != nil {
		a.fail(err.Error())
		return
	}
	mem := a.cfg.MaxMemoryMB
	if a.cfg.MemoryAuto {
		mem = java.AutoMemoryMB()
		a.cfg.MaxMemoryMB = mem
		a.setInt("memoryMB", mem)
	}
	bin, err := java.Find(a.cfg.JavaPath)
	if err != nil {
		a.fail("没有找到java路径,请手动指定javaw.exe\n如果已经安装过java,请选择手动指定,并点击浏览按钮")
		return
	}
	a.cfg.JavaPath = bin
	a.setStr("javaPath", bin)
	plan, err := launch.Build(launch.Request{
		VersionJSON:     inst.JSON,
		GameDir:         a.cfg.GameDir,
		JavaBin:         bin,
		MemoryMB:        mem,
		Session:         sess,
		Isolate:         a.cfg.IsolateGame,
		AuthlibInjector: a.cfg.AuthlibAgent,
		AuthServer:      a.cfg.YggdrasilRoot,
	})
	if err != nil {
		a.fail(err.Error())
		return
	}
	if a.cfg.Debug {
		name, body := launch.Script(plan)
		_ = os.WriteFile(name, []byte(body), 0o755)
		a.note("调试模式已写出 " + name)
	}
	cmd := exec.Command(plan.Args[0], plan.Args[1:]...)
	cmd.Dir = plan.Dir
	if a.cfg.Debug {
		f, err := os.Create(plan.LogHint)
		if err == nil {
			cmd.Stdout = f
			cmd.Stderr = f
		}
		a.note("你现在运行在调试模式下,如游戏不能正常启动,请查看启动器同目录下生成的'输出日志.txt'")
	}
	if err := cmd.Start(); err != nil {
		a.fail("java路径有误,是否已经安装java?\n" + err.Error())
		return
	}
	a.setStr("log", strings.Join(plan.Args, " "))
	a.setStr("panel", "log")
	a.note(fmt.Sprintf("已启动 %s  (pid %d)", plan.VersionID, cmd.Process.Pid))
	go func() { _ = cmd.Wait() }()
}

func (a *App) onUpdate() {
	if _, ok := a.current(); !ok {
		a.setStr("panel", "download")
		a.note("请在下载中心里选择想要更新的游戏版本")
		return
	}
	a.onInstallSelected()
}

func (a *App) onInstallSelected() {
	inst, ok := a.current()
	if !ok {
		return
	}
	a.mu.Lock()
	m := a.remote
	a.mu.Unlock()
	for i := range m.Versions {
		if m.Versions[i].ID == inst.ID {
			a.install(m.Versions[i])
			return
		}
	}
	a.note("版本清单里没有 " + inst.ID + "，打开下载中心选一个官方版本")
	a.setStr("panel", "download")
}

func (a *App) onInstall() {
	rows := a.filteredRemote()
	idx := 0
	if v := a.bridge.Value("remoteIndex"); v != nil {
		idx = v.ToInt()
	}
	if idx < 0 || idx >= len(rows) {
		a.note("请在下载中心里选择想要更新的游戏版本")
		return
	}
	a.install(rows[idx])
}

func (a *App) install(target download.RemoteVersion) {
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.setBool("busy", true)
	a.setInt("progress", 0)
	a.note("正在更新 " + target.ID)
	gameDir := a.cfg.GameDir
	go func() {
		err := download.Install(ctx, gameDir, target, func(done, total int, label string) {
			pct := 0
			if total > 0 {
				pct = done * 100 / total
			}
			mainthread.Wait(func() {
				a.setInt("progress", pct)
				a.note(fmt.Sprintf("正在下载 %s  (%d/%d)", label, done, total))
			})
		})
		mainthread.Wait(func() {
			a.setBool("busy", false)
			if err != nil {
				a.fail("更新失败: " + err.Error())
				return
			}
			a.cfg.SelectedVersion = target.ID
			_ = config.Save(a.cfg)
			a.setInt("progress", 100)
			a.note("完成 " + target.ID)
			a.refreshLocal()
			a.setStr("panel", "")
		})
	}()
}

func (a *App) loadRemote() {
	m, err := download.FetchManifest(context.Background())
	mainthread.Wait(func() {
		if err != nil {
			a.note(err.Error())
			return
		}
		a.mu.Lock()
		a.remote = m
		a.mu.Unlock()
		a.refreshRemote()
		a.note("获取列表完成，最新正式版 " + m.Latest.Release)
	})
}

func (a *App) onOpenDir() {
	_ = os.MkdirAll(a.cfg.GameDir, 0o755)
	openDir(a.cfg.GameDir)
	a.note("打开目录 " + a.cfg.GameDir)
}

func (a *App) onSave() {
	if strings.TrimSpace(a.cfg.PlayerName) == "" {
		a.note("没有填写游戏名字")
		return
	}
	a.cfg.FirstRun = false
	if err := config.Save(a.cfg); err != nil {
		a.fail(err.Error())
		return
	}
	a.setBool("firstRun", false)
	a.setStr("panel", "")
	a.note("保存设置")
	a.refreshLocal()
}

func (a *App) onFindJava() {
	bin, err := java.Find("")
	if err != nil {
		a.fail(err.Error())
		return
	}
	a.cfg.JavaPath = bin
	a.setStr("javaPath", bin)
	a.note("找到java路径,已经自动填入")
}

func (a *App) onBrowseJava() {
	path := qt.QFileDialog_GetOpenFileName4(a.parent, "选择java路径", a.cfg.JavaPath, "Java (javaw.exe java.exe java);;All (*)")
	if path == "" {
		return
	}
	a.cfg.JavaPath = path
	a.setStr("javaPath", path)
}

func (a *App) onBrowseDir() {
	path := qt.QFileDialog_GetExistingDirectory3(a.parent, "游戏目录", a.cfg.GameDir)
	if path == "" {
		return
	}
	a.cfg.GameDir = path
	a.setStr("gameDir", path)
	a.refreshLocal()
}

func (a *App) onLogin() {
	ctx := context.Background()
	switch a.cfg.AuthMode {
	case "yggdrasil":
		sess, err := auth.Yggdrasil(ctx, a.cfg.YggdrasilRoot, a.cfg.Email, a.password)
		a.password = ""
		if err != nil {
			a.fail("测试登录失败\n" + err.Error())
			return
		}
		a.cfg.AccessToken = sess.AccessToken
		a.cfg.UUID = sess.UUID
		a.cfg.AuthName = sess.Name
		a.cfg.PlayerName = sess.Name
		a.setStr("playerName", sess.Name)
		_ = config.Save(a.cfg)
		a.note("测试登录成功 " + sess.Name)
	case "microsoft":
		dc, err := auth.StartMicrosoft(ctx)
		if err != nil {
			a.fail(err.Error())
			return
		}
		a.setStr("log", fmt.Sprintf("在浏览器打开\n%s\n输入代码 %s", dc.VerificationURI, dc.UserCode))
		a.setStr("panel", "log")
		a.note("等待微软登录")
		go func() {
			sess, err := auth.PollMicrosoft(context.Background(), dc)
			mainthread.Wait(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				a.cfg.AuthMode = "microsoft"
				a.cfg.AccessToken = sess.AccessToken
				a.cfg.RefreshToken = sess.RefreshToken
				a.cfg.UUID = sess.UUID
				a.cfg.AuthName = sess.Name
				a.cfg.PlayerName = sess.Name
				a.setStr("playerName", sess.Name)
				a.setStr("authMode", "microsoft")
				_ = config.Save(a.cfg)
				a.note("测试登录成功 " + sess.Name)
			})
		}()
	default:
		a.cfg.AuthMode = "offline"
		a.cfg.AccessToken = ""
		a.note("将使用游戏名字离线进入")
	}
}

func (a *App) onClearLogin() {
	a.cfg.AccessToken = ""
	a.cfg.RefreshToken = ""
	a.cfg.UUID = ""
	a.cfg.AuthName = ""
	a.cfg.AuthMode = "offline"
	a.setStr("authMode", "offline")
	a.password = ""
	_ = config.Save(a.cfg)
	a.note("删除成功")
}

func (a *App) onImport() {
	name := strings.TrimSpace(a.cfg.PlayerName)
	if name == "" {
		name = "imported"
	}
	inst, err := version.ImportLegacy(a.cfg.GameDir, name)
	if err != nil {
		a.fail(err.Error())
		return
	}
	a.cfg.SelectedVersion = inst.ID
	a.refreshLocal()
	a.note("已经导入 " + inst.ID)
}

func (a *App) onRemove() {
	inst, ok := a.current()
	if !ok {
		a.note("没有选择游戏,请选择或点击更新按钮更新游戏")
		return
	}
	if err := version.Remove(a.cfg.GameDir, inst.ID); err != nil {
		a.fail(err.Error())
		return
	}
	a.note("将会删除 .minecraft/versions/" + inst.ID)
	a.cfg.SelectedVersion = ""
	a.refreshLocal()
}

func (a *App) onRename() {
	if v := a.bridge.Value("renameText"); v != nil {
		a.applyRename(v.ToString())
	}
}

func (a *App) applyRename(next string) {
	inst, ok := a.current()
	if !ok {
		a.note("没有选择游戏")
		return
	}
	if err := version.Rename(a.cfg.GameDir, inst.ID, next); err != nil {
		a.fail(err.Error())
		return
	}
	a.cfg.SelectedVersion = strings.TrimSpace(next)
	a.refreshLocal()
	a.note("重命名")
}

func (a *App) session() (auth.Session, error) {
	switch a.cfg.AuthMode {
	case "microsoft":
		if a.cfg.AccessToken != "" && a.cfg.AuthName != "" {
			return auth.Session{Name: a.cfg.AuthName, UUID: a.cfg.UUID, AccessToken: a.cfg.AccessToken, UserType: "msa", RefreshToken: a.cfg.RefreshToken}, nil
		}
		return auth.Session{}, fmt.Errorf("还没有微软登录。打开设置，点正版登录。")
	case "yggdrasil":
		if a.cfg.AccessToken != "" && a.cfg.AuthName != "" {
			return auth.Session{Name: a.cfg.AuthName, UUID: a.cfg.UUID, AccessToken: a.cfg.AccessToken, UserType: "legacy"}, nil
		}
		return auth.Session{}, fmt.Errorf("还没有验证服务器登录。打开设置，填写邮箱和密码后点正版登录测试。")
	default:
		return auth.Offline(a.cfg.PlayerName)
	}
}

func (a *App) current() (version.Installed, bool) {
	if len(a.versions) == 0 {
		return version.Installed{}, false
	}
	idx := 0
	if v := a.bridge.Value("versionIndex"); v != nil {
		idx = v.ToInt()
	}
	if idx < 0 || idx >= len(a.versions) {
		return version.Installed{}, false
	}
	return a.versions[idx], true
}

func (a *App) selectVersion(idx int) {
	if idx < 0 || idx >= len(a.versions) {
		return
	}
	a.cfg.SelectedVersion = a.versions[idx].ID
}

func (a *App) refreshLocal() {
	list, err := version.Scan(a.cfg.GameDir)
	if err != nil {
		a.note(err.Error())
		list = nil
	}
	a.versions = list
	labels := make([]string, 0, len(list))
	sel := 0
	if len(list) == 0 {
		labels = []string{"第一次进入游戏"}
	}
	for i, v := range list {
		label := v.ID
		if v.Type != "" {
			label += "    " + typeName(v.Type)
		}
		labels = append(labels, label)
		if v.ID == a.cfg.SelectedVersion {
			sel = i
		}
	}
	a.setList("versionLabels", labels)
	if len(list) == 0 {
		sel = 0
	}
	a.setInt("versionIndex", sel)
	if version.HasLegacyBin(a.cfg.GameDir) {
		a.note("当前目录发现旧版本游戏,开始导入 可在设置里点导入旧版本")
	}
}

func (a *App) filteredRemote() []download.RemoteVersion {
	a.mu.Lock()
	m := a.remote
	a.mu.Unlock()
	want := "release"
	if v := a.bridge.Value("remoteFilter"); v != nil && v.ToString() != "" {
		want = v.ToString()
	}
	if want != "release" && want != "snapshot" {
		return nil
	}
	var out []download.RemoteVersion
	for _, v := range m.Versions {
		if v.Type != want {
			continue
		}
		out = append(out, v)
		if len(out) >= 80 {
			break
		}
	}
	return out
}

func (a *App) refreshRemote() {
	rows := a.filteredRemote()
	labels := make([]string, 0, len(rows))
	for _, v := range rows {
		labels = append(labels, v.ID+"    "+v.ReleaseTime)
	}
	filter := "release"
	if v := a.bridge.Value("remoteFilter"); v != nil && v.ToString() != "" {
		filter = v.ToString()
	}
	if filter != "release" && filter != "snapshot" {
		labels = []string{"mcsky 镜像已关闭，这里只安装官方版本"}
	}
	if len(labels) == 0 {
		labels = []string{"正在获取列表"}
	}
	a.setList("remoteLabels", labels)
}

func (a *App) note(s string) {
	a.setStr("status", s)
}

func (a *App) fail(s string) {
	a.setStr("status", s)
	a.setStr("log", s)
	a.setStr("panel", "log")
}

func (a *App) setStr(key, val string) {
	a.set(key, qt.NewQVariant14(val))
}

func (a *App) setInt(key string, val int) {
	a.set(key, qt.NewQVariant4(val))
}

func (a *App) setBool(key string, val bool) {
	a.set(key, qt.NewQVariant8(val))
}

func (a *App) setList(key string, val []string) {
	if val == nil {
		val = []string{}
	}
	a.set(key, qt.NewQVariant15(val))
}

func (a *App) set(key string, val *qt.QVariant) {
	a.writing = true
	a.bridge.Insert(key, val)
	a.writing = false
}

func typeName(t string) string {
	switch t {
	case "snapshot":
		return "快照预览版"
	case "release", "":
		return "正式版"
	default:
		return t
	}
}

func fileURL(path string) string {
	if path == "" {
		return ""
	}
	return qt.QUrl_FromLocalFile(path).ToString()
}

func openDir(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	_ = cmd.Start()
}
