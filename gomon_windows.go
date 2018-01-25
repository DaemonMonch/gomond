package main

import (
	"strconv"
	"os"
	"os/exec"
)


func doRestart(cmd ...string) (*exec.Cmd,error) {
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
	return cm,nil	
}

func killProcess(cmd *exec.Cmd){
	if cmd != nil {
		pid := cmd.Process.Pid
		cm := exec.Command("taskkill","/PID",strconv.Itoa(pid),"/T")
		err := cm.Run()
		if err != nil {
			d("kill fail %s",err)
		}
		
	}
}