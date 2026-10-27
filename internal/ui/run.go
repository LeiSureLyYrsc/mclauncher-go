// Package ui is the Qt Quick window. The pictures under assets/skins are the
// ones carved out of the 2014 launcher; QML only places them.
package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	go a.loadRemote()
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
	a.setStr("authlib", c.AuthlibAgent)
	a.setStr("authMode", c.AuthMode)
	a.setBool("firstRun", c.FirstRun)
	a.setStr("status", "就绪")
	a.setInt("progress", 0)
	a.setBool("busy", false)
	a.setStr("panel", "")
	a.setStr("log", "")
	a.setStr("remoteFilter", "all")
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
	case "authlib":
		a.cfg.AuthlibAgent = value.ToString()
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
	case "browseAuthlib":
		a.onBrowseAuthlib()
	case "fetchAuthlib":
		a.onFetchAuthlib()
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
	if strings.TrimSpace(a.cfg.PlayerName) == "" {
		a.cfg.PlayerName = "Player"
		a.setStr("playerName", "Player")
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
	want := 0
	if ver, err := launch.LoadVersion(inst.JSON); err == nil && ver.JavaVersion != nil {
		want = ver.JavaVersion.MajorVersion
	}
	bin, err := java.Choose(a.cfg.JavaPath, a.cfg.GameDir, want)
	if err != nil {
		a.fail("没有找到java路径,请手动指定javaw.exe\n如果已经安装过java,请选择手动指定,并点击浏览按钮\n" + err.Error())
		return
	}
	if strings.TrimSpace(a.cfg.JavaPath) == "" {
		a.cfg.JavaPath = bin
		a.setStr("javaPath", bin)
	}
	agent, server := "", ""
	if a.cfg.AuthMode == "authlib" || a.cfg.AuthMode == "yggdrasil" {
		jar, err := a.ensureAgent()
		if err != nil {
			a.fail(err.Error())
			return
		}
		if strings.TrimSpace(a.cfg.YggdrasilRoot) == "" {
			a.fail("外置登录需要填写验证服务器，例如 https://littleskin.cn/api/yggdrasil")
			return
		}
		agent, server = jar, a.cfg.YggdrasilRoot
	}
	plan, err := launch.Build(launch.Request{
		VersionJSON:     inst.JSON,
		GameDir:         a.cfg.GameDir,
		JavaBin:         bin,
		MemoryMB:        mem,
		Session:         sess,
		Isolate:         a.cfg.IsolateGame,
		AuthlibInjector: agent,
		AuthServer:      server,
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
	list := java.Sniff(a.cfg.GameDir, a.cfg.JavaPath)
	if len(list) == 0 {
		a.fail("没有找到 java。把路径填到设置里，或装一个 JDK 后再点查找。")
		return
	}
	var b strings.Builder
	for _, rt := range list {
		fmt.Fprintf(&b, "Java %d    %s\n%s\n\n", rt.Major, rt.Version, rt.Path)
	}
	a.cfg.JavaPath = list[0].Path
	a.setStr("javaPath", list[0].Path)
	a.setStr("log", strings.TrimSpace(b.String()))
	a.setStr("panel", "log")
	a.note(fmt.Sprintf("找到 %d 个 Java，已填入 Java %d", len(list), list[0].Major))
}

func (a *App) onBrowseJava() {
	path := qt.QFileDialog_GetOpenFileName4(a.parent, "选择java路径", a.cfg.JavaPath, "Java (javaw.exe java.exe java);;All (*)")
	if path == "" {
		return
	}
	a.cfg.JavaPath = path
	a.setStr("javaPath", path)
}

func (a *App) onBrowseAuthlib() {
	path := qt.QFileDialog_GetOpenFileName4(a.parent, "Authlib Injector", a.cfg.AuthlibAgent, "Jar (*.jar);;All (*)")
	if path == "" {
		return
	}
	a.cfg.AuthlibAgent = path
	a.setStr("authlib", path)
}

func (a *App) onFetchAuthlib() {
	jar, err := a.ensureAgent()
	if err != nil {
		a.fail(err.Error())
		return
	}
	a.note("Authlib Injector 已就绪 " + jar)
}

func (a *App) ensureAgent() (string, error) {
	destDir, err := config.Dir()
	if err != nil {
		return "", err
	}
	jar, err := auth.EnsureAuthlib(context.Background(), a.cfg.AuthlibAgent, filepath.Join(destDir, "authlib-injector.jar"))
	if err != nil {
		return "", err
	}
	a.cfg.AuthlibAgent = jar
	a.setStr("authlib", jar)
	return jar, nil
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
	case "authlib", "yggdrasil":
		if strings.TrimSpace(a.cfg.YggdrasilRoot) == "" {
			a.fail("外置登录需要填写验证服务器，例如 https://littleskin.cn/api/yggdrasil")
			return
		}
		if _, err := a.ensureAgent(); err != nil {
			a.fail(err.Error())
			return
		}
		sess, err := auth.Yggdrasil(ctx, a.cfg.YggdrasilRoot, a.cfg.Email, a.password)
		a.password = ""
		if err != nil {
			a.fail("外置登录失败\n" + err.Error())
			return
		}
		a.cfg.AuthMode = "authlib"
		a.cfg.AccessToken = sess.AccessToken
		a.cfg.UUID = sess.UUID
		a.cfg.AuthName = sess.Name
		a.cfg.PlayerName = sess.Name
		a.setStr("playerName", sess.Name)
		a.setStr("authMode", "authlib")
		_ = config.Save(a.cfg)
		a.note("外置登录成功 " + sess.Name)
	case "microsoft":
		a.note("微软登录稍后实现")
	default:
		if strings.TrimSpace(a.cfg.PlayerName) == "" {
			a.cfg.PlayerName = "Player"
			a.setStr("playerName", "Player")
		}
		a.cfg.AuthMode = "offline"
		a.cfg.AccessToken = ""
		a.setStr("authMode", "offline")
		a.note("离线登录，名字 " + a.cfg.PlayerName)
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
	case "authlib", "yggdrasil":
		if a.cfg.AccessToken != "" && a.cfg.AuthName != "" {
			return auth.Session{Name: a.cfg.AuthName, UUID: a.cfg.UUID, AccessToken: a.cfg.AccessToken, UserType: "legacy"}, nil
		}
		return auth.Session{}, fmt.Errorf("还没有外置登录。打开设置，填写验证服务器、账号和密码。")
	case "microsoft":
		return auth.Session{}, fmt.Errorf("微软登录稍后实现")
	default:
		name := strings.TrimSpace(a.cfg.PlayerName)
		if name == "" {
			name = "Player"
		}
		return auth.Offline(name)
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
	want := "all"
	if v := a.bridge.Value("remoteFilter"); v != nil && v.ToString() != "" {
		want = v.ToString()
	}
	var out []download.RemoteVersion
	for _, v := range m.Versions {
		if !matchFilter(want, v.Type) {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (a *App) refreshRemote() {
	rows := a.filteredRemote()
	labels := make([]string, 0, len(rows))
	for _, v := range rows {
		labels = append(labels, v.ID+"    "+typeName(v.Type)+"    "+v.ReleaseTime)
	}
	if len(labels) == 0 {
		labels = []string{"正在获取列表"}
	}
	a.setList("remoteLabels", labels)
}

func matchFilter(want, typ string) bool {
	switch want {
	case "", "all":
		return true
	case "old":
		return typ == "old_alpha" || typ == "old_beta"
	default:
		return typ == want
	}
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
	case "old_beta":
		return "远古Beta"
	case "old_alpha":
		return "远古Alpha"
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
