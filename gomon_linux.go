
package main

import (
	"syscall"
	"os/exec"
)



func doRestart(cmd ...string) (*exec.Cmd,error) {
	cm := exec.Command("go",cmd...)
	d("restart ")
	cm.Stdout = os.Stdout
	cm.Stdin = os.Stdin
	cm.Stderr = os.Stderr
	cm.SysProcAttr = &syscall.SysProcAttr{Setpgid:true}
	err := cm.Start()
	if err != nil {
		d("restart fail! %s",err)
		return nil,err
	}
	return cm,nil	
}

func killProcess(cmd *exec.Cmd){
	if cmd != nil {
		pgid,err := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			d("Get pgid err %s",err)
		}
		if err = syscall.Kill(-pgid,syscall.SIGKILL);err != nil{
			d("kill err %s" ,err)
		}
		<- time.After(2 * time.Second)
	}
}