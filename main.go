package main

import (
	"os/signal"
	 "time"
	"context"
	"os/exec"
	"strings"
	"flag"
	"path/filepath"
	"fmt"
	"github.com/howeyc/fsnotify"
	"os"
	"sync/atomic"

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
	done := make(chan os.Signal,1)
	restartChan := make(chan interface{})

	startRestartWatchDog(restartChan,context.WithValue(ctx,"cmd",cmds))
	// Process events
	go func() {
		filters := strings.Split(*includeFileExts,",")
		filters = append(filters,"go")
		for {
			select {
			case ev := <-watcher.Event:
				onrestarting := restarting.Load().(bool)
				if(!onrestarting){
					d("event: %v", ev)
					restarting.Store(true)
					restartChan <- 1
				}
				
			case <-watcher.Error:
				//d("error: %v", err)
			}
		}
	}()
	

	watchSourceFiles(watcher,wd)
	
	signal.Notify(done,os.Interrupt,os.Kill)
	<- done
	d("signal revieve ")
	/* ... do stuff ... */
	watcher.Close()
	cancelFunc()
	<- restartChan	
}



func startRestartWatchDog(restart chan interface{},ctx context.Context){
	go func() {
		var cmd *exec.Cmd
		for {
			select{
				case <-restart:
					 killProcess(cmd)
					c,err := doRestart(ctx.Value("cmd").([]string)...)
					if err == nil {
						d("running pid %d",c.Process.Pid)
						cmd = c
					}else{
						cmd = nil
					}
					<- time.After(5 * time.Second)
					restarting.Store(false) 
				case <-ctx.Done():
					 d("watchdog exit")
					killProcess(cmd)
					close(restart)
					return 
			}

		}
	}()
}



func watchSourceFiles(watcher *fsnotify.Watcher,root string){
	filepath.Walk(root,func(path string, info os.FileInfo, err error) error{
		if err != nil {
			d("error occur on dir %s cause %s",path,err)
			return err
		}

		
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			
			d("watching %s",path)
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