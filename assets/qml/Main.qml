import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window

Window {
    id: root
    width: 720
    height: 450
    visible: true
    title: "忘却的旋律"
    color: "#000000"
    flags: Qt.FramelessWindowHint | Qt.Window

    readonly property string yahei: "Microsoft YaHei"

    Image {
        id: wallpaper
        anchors.fill: parent
        source: app.skinUrl
        fillMode: Image.Stretch
        asynchronous: true
    }

    MouseArea {
        z: 0
        anchors.fill: parent
        property point origin
        onPressed: function (mouse) { origin = Qt.point(mouse.x, mouse.y) }
        onPositionChanged: function (mouse) {
            if (pressed)
                root.setX(root.x + mouse.x - origin.x), root.setY(root.y + mouse.y - origin.y)
        }
    }

    Rectangle { anchors.fill: parent; color: "transparent"; border.color: "#33000000"; border.width: 1 }

    Text {
        x: 10
        y: 8
        text: "MineCraft Launcher"
        color: "white"
        font.family: root.yahei
        font.pixelSize: 13
        style: Text.Outline
        styleColor: "#000000"
    }

    Row {
        z: 10
        anchors.right: parent.right
        anchors.top: parent.top
        spacing: 0
        Image {
            width: 38
            height: 38
            source: minArea.containsMouse ? app.assetsUrl + "/chrome/min-b.png" : app.assetsUrl + "/chrome/min-a.png"
            MouseArea {
                id: minArea
                anchors.fill: parent
                hoverEnabled: true
                onClicked: root.showMinimized()
            }
        }
        Image {
            width: 38
            height: 38
            source: closeArea.containsMouse ? app.assetsUrl + "/chrome/close-b.png" : app.assetsUrl + "/chrome/close-a.png"
            MouseArea {
                id: closeArea
                anchors.fill: parent
                hoverEnabled: true
                onClicked: Qt.quit()
            }
        }
    }

    function placeDialog(win) {
        var s = root.screen
        if (!s)
            return
        win.screen = s
        var ax = s.virtualX
        var ay = s.virtualY
        var aw = s.desktopAvailableWidth > 0 ? s.desktopAvailableWidth : s.width
        var ah = s.desktopAvailableHeight > 0 ? s.desktopAvailableHeight : s.height
        if (win.width > aw)
            win.width = Math.max(win.minimumWidth, aw)
        if (win.height > ah - 8)
            win.height = Math.max(win.minimumHeight, ah - 8)
        var c = root.mapToGlobal(root.width / 2, root.height / 2)
        var x = c.x - win.width / 2
        var y = c.y - win.height / 2
        if (x < ax)
            x = ax
        if (y < ay)
            y = ay
        if (x + win.width > ax + aw)
            x = Math.max(ax, ax + aw - win.width)
        if (y + win.height > ay + ah)
            y = Math.max(ay, ay + ah - win.height)
        win.x = x
        win.y = y
    }

    component GlassButton: Button {
        font.family: root.yahei
        font.pixelSize: 13
        hoverEnabled: true
        background: Rectangle {
            radius: 2
            color: parent.down ? "#d2d2d2" : (parent.hovered ? "#ffffff" : "#e8e8e8")
            opacity: parent.enabled ? 0.82 : 0.55
            border.width: 1
            border.color: "#f4f4f4"
        }
        contentItem: Text {
            text: parent.text
            color: "#1a1a1a"
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
            font: parent.font
        }
    }

    GlassButton {
        x: 420
        y: 292
        width: 162
        height: 56
        text: "启动游戏"
        font.pixelSize: 18
        enabled: !app.busy
        onClicked: app.cmd = "launch"
    }
    GlassButton {
        x: 594
        y: 292
        width: 112
        height: 24
        text: "更新游戏"
        enabled: !app.busy
        onClicked: app.cmd = "update"
    }
    GlassButton {
        x: 594
        y: 320
        width: 112
        height: 24
        text: "下载中心"
        onClicked: app.panel = "download"
    }
    GlassButton {
        x: 594
        y: 348
        width: 112
        height: 24
        text: "设置"
        onClicked: app.panel = "setup"
    }

    Column {
        x: 436
        y: 376
        width: 200
        spacing: 2
        Text {
            text: "选中的版本"
            color: "white"
            font.family: root.yahei
            font.pixelSize: 12
            style: Text.Outline
            styleColor: "#000000"
        }
        ComboBox {
            id: ver
            width: 200
            height: 22
            model: app.versionLabels
            font.family: root.yahei
            font.pixelSize: 12
            onActivated: function (i) { app.versionIndex = i }
            Connections {
                target: app
                function onValueChanged(key, value) {
                    if (key === "versionLabels" || key === "versionIndex")
                        ver.currentIndex = app.versionIndex
                }
            }
        }
    }

    Label {
        x: 648
        y: 396
        text: "打开目录"
        color: "#2410c8"
        font.family: root.yahei
        font.pixelSize: 13
        font.underline: true
        background: Rectangle { color: "#ccffffff" }
        MouseArea { anchors.fill: parent; cursorShape: Qt.PointingHandCursor; onClicked: app.cmd = "openDir" }
    }

    Row {
        x: 10
        y: 424
        spacing: 8
        Text {
            text: "MineCraft Launcher"
            color: "white"
            font.family: root.yahei
            font.pixelSize: 12
            style: Text.Outline
            styleColor: "#000000"
        }
        Text {
            text: "by 忘却的旋律"
            color: "white"
            font.family: root.yahei
            font.pixelSize: 12
            font.underline: true
            style: Text.Outline
            styleColor: "#000000"
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: Qt.openUrlExternally("http://www.mcsky.net")
            }
        }
    }

    Rectangle {
        visible: app.status !== "就绪" || app.busy
        x: 10
        y: 400
        width: statusText.implicitWidth + 12
        height: 18
        radius: 2
        color: "#99000000"
        Text {
            id: statusText
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 6
            text: app.status
            color: "white"
            font.family: root.yahei
            font.pixelSize: 12
        }
    }

    Rectangle {
        visible: app.busy
        x: 10
        y: 388
        width: 280
        height: 6
        color: "#66000000"
        Rectangle {
            width: parent.width * Math.max(0, Math.min(100, app.progress)) / 100
            height: parent.height
            color: "#b6e400"
        }
    }

    Window {
        id: setup
        width: 560
        height: 560
        title: app.firstRun ? "请完成以下设置" : "游戏设置窗口"
        transientParent: root
        flags: Qt.Window | Qt.WindowTitleHint | Qt.WindowSystemMenuHint | Qt.WindowCloseButtonHint
        minimumWidth: 480
        minimumHeight: 360
        visible: app.panel === "setup"
        color: "#e7eeef"
        onVisibleChanged: if (visible) root.placeDialog(setup)
        onClosing: function (close) { app.panel = "" }

        Rectangle {
            id: setupBanner
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: 42
            color: "#d5e0e2"
            z: 2
            Text {
                anchors.left: parent.left
                anchors.leftMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                text: setup.title
                font.family: root.yahei
                font.pixelSize: 14
                color: "#1a1a1a"
            }
            Image {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: 38
                height: 38
                source: setupClose.containsMouse ? app.assetsUrl + "/chrome/close-b.png" : app.assetsUrl + "/chrome/close-a.png"
                MouseArea {
                    id: setupClose
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: app.panel = ""
                }
            }
        }

        Flickable {
            anchors.top: setupBanner.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            contentHeight: form.implicitHeight + 24
            clip: true
            Column {
                id: form
                x: 16
                y: 12
                width: parent.width - 32
                spacing: 8

                Grid {
                    width: parent.width
                    columns: 2
                    columnSpacing: 12
                    rowSpacing: 8
                    Label { text: "游戏名字"; font.bold: true }
                    Label { text: "最大内存"; font.bold: true }
                    TextField {
                        id: nameField
                        width: 250
                        Component.onCompleted: text = app.playerName
                        onTextEdited: app.playerName = text
                    }
                    TextField {
                        id: memField
                        width: 250
                        enabled: !app.memoryAuto
                        Component.onCompleted: text = String(app.memoryMB)
                        onTextEdited: app.memoryMB = parseInt(text) || 1024
                    }
                    Label { text: "内存设置方式"; font.bold: true }
                    Label { text: "启动器皮肤"; font.bold: true }
                    ComboBox {
                        width: 250
                        model: ["自动", "手动"]
                        currentIndex: app.memoryAuto ? 0 : 1
                        onActivated: function (i) { app.memoryAuto = i === 0 }
                    }
                    ComboBox {
                        width: 250
                        model: ["默认皮肤", "沙漠", "晴空"]
                        currentIndex: app.skin === "default" ? 1 : (app.skin === "sky" ? 2 : 0)
                        onActivated: function (i) { app.skin = ["sunset", "default", "sky"][i] }
                    }
                }

                Label {
                    width: parent.width
                    wrapMode: Text.WordWrap
                    text: "按系统剩余内存，最大 1300M，最小 500M。如需要更大的内存,请手动设置。"
                    color: "#355"
                }
                Label { text: "java路径（留空则按版本自动嗅探）"; font.bold: true }
                Row {
                    spacing: 6
                    TextField {
                        id: javaField
                        width: 320
                        placeholderText: "自动嗅探"
                        Component.onCompleted: text = app.javaPath
                        onTextEdited: app.javaPath = text
                    }
                    Button { text: "浏览"; onClicked: app.cmd = "browseJava" }
                    Button { text: "查找java"; onClicked: app.cmd = "findJava" }
                }
                Label { text: "游戏目录"; font.bold: true }
                Row {
                    spacing: 6
                    TextField {
                        id: dirField
                        width: 390
                        Component.onCompleted: text = app.gameDir
                        onTextEdited: app.gameDir = text
                    }
                    Button { text: "浏览"; onClicked: app.cmd = "browseDir" }
                }
                ComboBox {
                    width: parent.width
                    model: ["保存到.minecraft", "保存到.minecraft/versions/版本名称/"]
                    currentIndex: app.isolate ? 1 : 0
                    onActivated: function (i) { app.isolate = i === 1 }
                }
                Label { text: "只在1.6以上的版本有效"; color: "#355" }
                Row {
                    spacing: 8
                    CheckBox { checked: app.debug; onToggled: app.debug = checked }
                    Label { text: "调试模式"; anchors.verticalCenter: parent.verticalCenter }
                }
                Label { text: "开启调试模式将以java.exe启动minecraft,并且在启动器目录生成输出日志.txt"; color: "#355"; width: parent.width; wrapMode: Text.WordWrap }
                Label { text: "登录"; font.bold: true }
                Label { text: "不填任何账号就是离线。外置登录走 Authlib Injector。微软登录稍后做。"; color: "#355"; width: parent.width; wrapMode: Text.WordWrap }
                ComboBox {
                    id: authMode
                    width: parent.width
                    model: ["离线", "外置登录"]
                    currentIndex: app.authMode === "authlib" || app.authMode === "yggdrasil" ? 1 : 0
                    onActivated: function (i) { app.authMode = i === 1 ? "authlib" : "offline" }
                }
                Column {
                    width: parent.width
                    spacing: 8
                    visible: authMode.currentIndex === 1
                    TextField {
                        width: parent.width
                        placeholderText: "验证服务器，例如 https://littleskin.cn/api/yggdrasil"
                        Component.onCompleted: text = app.yggdrasil
                        onTextEdited: app.yggdrasil = text
                    }
                    TextField {
                        width: parent.width
                        placeholderText: "账号邮箱"
                        Component.onCompleted: text = app.email
                        onTextEdited: app.email = text
                    }
                    TextField { width: parent.width; placeholderText: "密码（不会保存）"; echoMode: TextInput.Password; onTextEdited: app.password = text }
                    Row {
                        spacing: 6
                        TextField {
                            id: authlibField
                            width: 280
                            placeholderText: "authlib-injector.jar，可留空自动下载"
                            Component.onCompleted: text = app.authlib
                            onTextEdited: app.authlib = text
                        }
                        Button { text: "浏览"; onClicked: app.cmd = "browseAuthlib" }
                        Button { text: "下载"; onClicked: app.cmd = "fetchAuthlib" }
                    }
                    Button { text: "外置登录"; onClicked: app.cmd = "login" }
                }
                Button { text: "清除登录，改回离线"; onClicked: app.cmd = "clearLogin" }
                Row {
                    spacing: 6
                    TextField { id: renameField; width: 220; placeholderText: "输入版本名称" }
                    Button { text: "重命名"; onClicked: app.cmd = "rename:" + renameField.text }
                    Button { text: "删除选中游戏"; onClicked: app.cmd = "removeVersion" }
                    Button { text: "导入旧版本"; onClicked: app.cmd = "importLegacy" }
                }
                Row {
                    spacing: 6
                    Button { text: "保存设置"; onClicked: app.cmd = "save" }
                    Button { text: "完成设置"; onClicked: app.cmd = "save" }
                    Button { text: "取消"; onClicked: app.panel = "" }
                }
            }
        }

        Connections {
            target: app
            function onValueChanged(key, value) {
                if (key === "playerName" && nameField.text !== String(value))
                    nameField.text = value
                if (key === "javaPath" && javaField.text !== String(value))
                    javaField.text = value
                if (key === "authlib" && authlibField.text !== String(value))
                    authlibField.text = value
                if (key === "gameDir" && dirField.text !== String(value))
                    dirField.text = value
                if (key === "memoryMB" && memField.text !== String(value))
                    memField.text = value
            }
        }
    }

    Window {
        id: down
        width: 640
        height: 460
        title: "下载中心"
        transientParent: root
        flags: Qt.Window | Qt.WindowTitleHint | Qt.WindowSystemMenuHint | Qt.WindowCloseButtonHint
        minimumWidth: 480
        minimumHeight: 320
        visible: app.panel === "download"
        color: "#e7eeef"
        onVisibleChanged: if (visible) root.placeDialog(down)
        onClosing: function (close) { app.panel = "" }

        Rectangle {
            id: downBanner
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: 42
            color: "#d5e0e2"
            z: 2
            Text {
                anchors.left: parent.left
                anchors.leftMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                text: "下载中心"
                font.family: root.yahei
                font.pixelSize: 14
            }
            Image {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: 38
                height: 38
                source: downClose.containsMouse ? app.assetsUrl + "/chrome/close-b.png" : app.assetsUrl + "/chrome/close-a.png"
                MouseArea {
                    id: downClose
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: app.panel = ""
                }
            }
        }

        ColumnLayout {
            anchors.top: downBanner.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: 12
            spacing: 8
            Row {
                spacing: 6
                ComboBox {
                    id: sourceBox
                    width: 140
                    model: ["官方", "BMCLAPI"]
                    currentIndex: app.downloadSource === "bmclapi" ? 1 : 0
                    onActivated: function (i) { app.downloadSource = i === 1 ? "bmclapi" : "official" }
                }
                Button { text: "全部版本"; onClicked: app.remoteFilter = "all" }
                Button { text: "正式版"; onClicked: app.remoteFilter = "release" }
                Button { text: "快照"; onClicked: app.remoteFilter = "snapshot" }
                Button { text: "远古"; onClicked: app.remoteFilter = "old" }
                Button { text: "刷新"; onClicked: app.cmd = "reloadRemote" }
            }
            ListView {
                id: remoteList
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                model: app.remoteLabels
                currentIndex: app.remoteIndex
                delegate: ItemDelegate {
                    width: remoteList.width
                    text: modelData
                    highlighted: remoteList.currentIndex === index
                    onClicked: app.remoteIndex = index
                }
            }
            Label {
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
                color: "#355"
                text: app.downloadSource === "bmclapi"
                      ? "BMCLAPI：国内镜像。文件会跳到志愿节点，节点连不上会自动换节点，再不行改走官方。短时间请求太多会被限流。"
                      : "官方源：Mojang 原站。国内直连可能很慢或被断开，可以改成 BMCLAPI。"
            }
            Row {
                spacing: 6
                Button { text: "确定"; enabled: !app.busy; onClicked: app.cmd = "install" }
                Button { text: "刷新列表"; onClicked: app.cmd = "reloadRemote" }
                Button { text: "取消"; onClicked: app.panel = "" }
            }
        }
    }

    Window {
        id: logWin
        width: 560
        height: 320
        title: app.debug ? "输出日志.txt" : "提示"
        transientParent: root
        flags: Qt.Window | Qt.WindowTitleHint | Qt.WindowSystemMenuHint | Qt.WindowCloseButtonHint
        minimumWidth: 360
        minimumHeight: 220
        visible: app.panel === "log"
        color: "#e7eeef"
        onVisibleChanged: if (visible) root.placeDialog(logWin)
        onClosing: function (close) { app.panel = "" }
        Rectangle {
            id: logBanner
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: 42
            color: "#d5e0e2"
            z: 2
            Text {
                anchors.left: parent.left
                anchors.leftMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                text: logWin.title
                font.family: root.yahei
                font.pixelSize: 14
            }
            Image {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: 38
                height: 38
                source: logClose.containsMouse ? app.assetsUrl + "/chrome/close-b.png" : app.assetsUrl + "/chrome/close-a.png"
                MouseArea {
                    id: logClose
                    anchors.fill: parent
                    hoverEnabled: true
                    onClicked: app.panel = ""
                }
            }
        }
        ColumnLayout {
            anchors.top: logBanner.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: 12
            spacing: 8
            ScrollView {
                Layout.fillWidth: true
                Layout.fillHeight: true
                TextArea {
                    text: app.log
                    readOnly: true
                    wrapMode: TextArea.Wrap
                    font.family: "Consolas"
                }
            }
            Button { text: "确定"; onClicked: app.panel = "" }
        }
    }
}
