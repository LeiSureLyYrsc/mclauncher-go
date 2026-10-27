// Command mclauncher-cli is the headless slice of the launcher: scan versions,
// import the old bin/ layout, and print the java command. The Qt window lives
// in cmd/mclauncher and needs a Qt 6 toolchain.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/auth"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/java"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/launch"
	"github.com/LeiSureLyYrsc/mclauncher-go/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scan":
		fs := flag.NewFlagSet("scan", flag.ExitOnError)
		dir := fs.String("dir", defaultGame(), "游戏目录")
		_ = fs.Parse(os.Args[2:])
		list, err := version.Scan(*dir)
		if err != nil {
			fatal(err)
		}
		if version.HasLegacyBin(*dir) {
			fmt.Println("当前目录发现旧版本游戏 (bin/minecraft.jar)")
		}
		if len(list) == 0 {
			fmt.Println("没有已安装版本")
			return
		}
		for _, v := range list {
			fmt.Printf("%s\t%s\n", v.ID, v.Type)
		}
	case "import":
		fs := flag.NewFlagSet("import", flag.ExitOnError)
		dir := fs.String("dir", defaultGame(), "游戏目录")
		name := fs.String("name", "old", "版本名称")
		_ = fs.Parse(os.Args[2:])
		inst, err := version.ImportLegacy(*dir, *name)
		if err != nil {
			fatal(err)
		}
		fmt.Println("完成", inst.ID)
	case "command":
		fs := flag.NewFlagSet("command", flag.ExitOnError)
		dir := fs.String("dir", defaultGame(), "游戏目录")
		id := fs.String("version", "", "版本 id")
		name := fs.String("name", "Player", "离线名字")
		mem := fs.Int("mem", 1024, "最大内存 MB")
		javaPath := fs.String("java", "", "java 路径")
		_ = fs.Parse(os.Args[2:])
		if *id == "" {
			fatal(fmt.Errorf("需要 --version"))
		}
		bin, err := java.Find(*javaPath)
		if err != nil {
			bin = "java"
		}
		sess, err := auth.Offline(*name)
		if err != nil {
			fatal(err)
		}
		plan, err := launch.Build(launch.Request{
			VersionJSON: filepath.Join(*dir, "versions", *id, *id+".json"),
			GameDir:     *dir,
			JavaBin:     bin,
			MemoryMB:    *mem,
			Session:     sess,
		})
		if err != nil {
			fatal(err)
		}
		for _, a := range plan.Args {
			fmt.Println(a)
		}
	case "java":
		p, err := java.Find("")
		if err != nil {
			fatal(err)
		}
		fmt.Println(p)
		fmt.Printf("自动内存 %d MB\n", java.AutoMemoryMB())
	default:
		usage()
		os.Exit(2)
	}
}

func defaultGame() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".minecraft")
}

func usage() {
	fmt.Fprintf(os.Stderr, "用法: mclauncher-cli scan|import|command|java\n")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
