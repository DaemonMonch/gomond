package main

import (
	"strings"
	"runtime"
	"context"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync/atomic"
)

var includeFileExts = flag.String("i", "", "include")
var buildRestarting atomic.Value
var cmd *exec.Cmd
var runningChan = make(chan interface{},1)

func main() {
	flag.Parse()
	d("i %s", *includeFileExts)
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	pkgName := filepath.Base(wd)

	buildRestarting.Store(false)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	done := make(chan os.Signal, 1)
	buildingChan := make(chan interface{})


	autoload(genBuildName(pkgName))
	startRestartWatchDog(buildingChan,context.WithValue(ctx,"name",pkgName))
	// Process events
	go func() {
		//filters := strings.Split(*includeFileExts, ",")
		//filters = append(filters, "go")
		for {
			select {
			case ev := <-watcher.Events:
				onrestarting := buildRestarting.Load().(bool)
				if(!onrestarting){
					d("event: %v", ev)
					buildRestarting.Store(true)
					buildingChan <- 1
				}

			case <-watcher.Errors:
				//d("error: %v", err)
			}
		}
	}()

	//watcher.Add(wd)
	watchSourceFiles(watcher, wd)

	signal.Notify(done, os.Interrupt, os.Kill)
	<-done
	d("signal revieve ")
	/* ... do stuff ... */
	watcher.Close()
	cancelFunc()
	<- buildingChan
}

func startRestartWatchDog(restart chan interface{}, ctx context.Context) {
	go func() {
		buildName := ctx.Value("name").(string)
		
		for {
			select {
			case <-restart:
				autoload(genBuildName(buildName))
				buildRestarting.Store(false)
			case <-ctx.Done():
				d("watchdog exit")
				kill()
				close(restart)
				return
			}

		}
	}()
}

func autoload(buildName string) {
	err := build(buildName)
	if err != nil {
		d("build err %s wait for next change",err)
		return
	}
	err = kill()
	if err != nil {
		d("kill err %s wait for next change",err)
		return
	}
	err = start(buildName)
	if err != nil {
		d("start err %s wait for next change",err)
		return
	}
}

func restart(buildName string) error {
	err := kill()
	if err != nil {
		return err
	}
	err = start(buildName)
	if err != nil {
		return err
	}
	return nil
}

func genBuildName(name string) string {
	if runtime.GOOS == "windows" {
		name = name + ".exe"
	}
	return name
}

func start(buildName string) error{
	if strings.Index(buildName,"./") != 0 {
		buildName = "./" + buildName
	}
	cmd = exec.Command(buildName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err == nil {
		d("start pid %d",cmd.Process.Pid)
	}
	go func(){
		cmd.Wait()
		runningChan <- 1
	}()
	return err	
}

func kill() error{
	if cmd != nil && cmd.Process != nil {
		d("kill  %d",cmd.Process.Pid)
		if err := cmd.Process.Kill();err != nil {
			d("kill fail %s",err)
			return err
		}
		<- runningChan
	}
	return nil
	
}

func build(buildName string) error{
	
	d("build %s",buildName)
	cmd := exec.Command("go","build","-o",buildName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func watchSourceFiles(watcher *fsnotify.Watcher, root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			d("error occur on dir %s cause %s", path, err)
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".go" {

			d("watching %s", path)
			err := watcher.Add(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func d(s string, i ...interface{}) {
	fmt.Printf("[DEBUG]\t"+s+"\n", i)
}
