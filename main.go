package main

import (
	"context"
	"os/exec"
	"strings"
	"flag"
	"path/filepath"
	"fmt"
	"github.com/howeyc/fsnotify"
	"os"
	"sync/atomic"
	//"os/exec"
)

var includeFileExts = flag.String("i","","include")
var restarting atomic.Value
func main() {
	flag.Parse()
	d("i %s",*includeFileExts)
	cmds := flag.Args()
	d("%v",cmds)
	if len(cmds) <= 0 {
		panic("run what?")
	}
	wd,err := os.Getwd()
	if err != nil {
		panic(err)
	}

	restarting.Store(false)

	watcher,err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	ctx,cancelFunc := context.WithCancel(context.Background())
	done := make(chan bool)
	restartChan := make(chan interface{})

	startRestartWatchDog(restartChan,context.WithValue(ctx,"cmd",cmds))
	// Process events
	go func() {
		filters := strings.Split(*includeFileExts,",")
		filters = append(filters,"go")
		for {
			select {
			case ev := <-watcher.Event:
				restarting := restarting.Load().(bool)
				if(!restarting){
					d("event: %v", ev)
					restartChan <- 1
				}
				
			case err := <-watcher.Error:
				d("error: %v", err)
			}
		}
	}()

	err = watcher.Watch(wd)
	if err != nil {
		panic(err)
	}

	watchSubDirs(watcher,wd)
	
	// Hang so program doesn't exit
	<-done

	/* ... do stuff ... */
	watcher.Close()
	cancelFunc()

}

func doRestart(cmd ...string) (*os.Process,error) {
	cm := exec.Command("go",cmd...)
	d("restart ")
	cm.Stdout = os.Stdout
	cm.Stdin = os.Stdin
	cm.Stderr = os.Stderr
	err := cm.Start()
	if err != nil {
		d("restart fail! %s",err)
		return nil,err
	}
	go func(){
		cm.Wait()
	}()
	return cm.Process,nil	
}

func startRestartWatchDog(restart <-chan interface{},ctx context.Context){
	go func() {
		var runningProcess *os.Process
		for {
			select{
				case <-restart:
					restarting.Store(true)
					if runningProcess != nil {
						err := runningProcess.Kill()
						if err != nil {
							d("kill fail %s",err)
						}
						runningProcess.Release()
						runningProcess.Wait()
					}
					p,err := doRestart(ctx.Value("cmd").([]string)...)
					if err == nil {
						d("running pid %d",p.Pid)
						runningProcess = p
					}else{
						runningProcess = nil
					}
					restarting.Store(false)
				case <-ctx.Done():
					if runningProcess != nil {
						runningProcess.Kill()
					}
					return
			}

		}
	}()
}

func watchSubDirs(watcher *fsnotify.Watcher,root string){
	filepath.Walk(root,func(path string, info os.FileInfo, err error) error{
		if err != nil {
			d("error occur on dir %s cause %s",path,err)
			return err
		}
		
		if info.IsDir() {
			//d("watching %s",path)
			err := watcher.Watch(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func d(s string,i ...interface{}){
	fmt.Printf("[DEBUG]\t" + s + "\n",i)
}