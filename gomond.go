package main

import (
	"sync"
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

var (
	includeFileExts = flag.String("i", "", "include")
	buildRestarting atomic.Value
	cmd *exec.Cmd
	l sync.Mutex
	done = make(chan os.Signal, 1)
	buildingChan = make(chan interface{})
)


func main() {
	flag.Parse()
	d("i %s", *includeFileExts)
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	pkgName := filepath.Base(wd)
	ctx, cancelFunc := context.WithCancel(context.Background())
	
	buildRestarting.Store(false)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	watchEventLoop(watcher,ctx)
	buildAndRestart(genBuildName(pkgName))
	startRestartWatchDog(buildingChan,context.WithValue(ctx,"name",pkgName))
	
	//watchSourceFiles(watcher, wd)
	watcher.Add(wd)
	wait()
	
	watcher.Close()
	cancelFunc()
	<- buildingChan
}

func wait(){ 
	signal.Notify(done, os.Interrupt, os.Kill)
	<-done
	d("signal revieve ")
}

func watchEventLoop(watcher *fsnotify.Watcher,ctx context.Context){
	go func() {
		//filters := strings.Split(*includeFileExts, ",")
		//filters = append(filters, "go")
		for {
			select {
			case ev := <-watcher.Events:

				if !isReloadEvent(ev){
					continue
				}
				
				if !isSourceFile(ev.Name){
					continue
				}
				d("event: %v", ev)
				buildingChan <- 1
			case <-watcher.Errors:
				//d("error: %v", err)
			case <-ctx.Done():
				d("event loop exit")
				return;
			}
		}
	}()
}

func isReloadEvent(evt fsnotify.Event) bool{
	return evt.Op == fsnotify.Write
}

func isSourceFile(f string) bool {
	return filepath.Ext(f) == ".go"
}

func startRestartWatchDog(restart chan interface{}, ctx context.Context) {
	go func() {
		buildName := ctx.Value("name").(string)
		for {
			select {
			case <-restart:
				
				buildAndRestart(genBuildName(buildName))
			case <-ctx.Done():
				d("watchdog exit")
				kill()
				close(restart)
				return
			}

		}
	}()
}


func buildAndRestart(buildName string) {
	err := build(buildName)
	if err != nil {
		d("build err %s wait for next change",err)
		return
	}
	err = kill()
	if err != nil {
		d("kill err %s",err)
		return
	}
	err = start(buildName)
	if err != nil {
		d("start err %s wait for next change",err)
		return
	}
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
		l.Lock()
		defer l.Unlock()
		cmd.Wait()
		cmd = nil
	}()
	return err	
}

func kill() error{
	l.Lock()
	defer l.Unlock()
	if cmd != nil && cmd.Process != nil {
		d("kill  %d",cmd.Process.Pid)
		if err := cmd.Process.Kill();err != nil {
			return err
		}
		
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
	fmt.Printf("[gomon]\t"+s+"\n", i)
}
