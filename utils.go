package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
)

func Util_runcommand(cmdline string) {
	com := exec.Command("sh", "-c", cmdline)
	comStdout, err := com.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := com.Start(); err != nil {
		log.Fatal(err)
	}
	output, _ := ioutil.ReadAll(comStdout)
	fmt.Printf("%s", output)
}

func Util_create_dbpath() {
	var cmdline string
	cmdline = "mkdir data"
	fmt.Println(cmdline)
}

func Util_ps() {
	cmdline := "ps -Ao 'pid,command' | grep -v 'grep .* mongod' | grep '\\smongo[ds]\\s'"
	Util_runcommand(cmdline)
}

func Util_kill() {
	cmdline := "pgrep mongo[d,s] | xargs -t kill"
	fmt.Println("Killing:")
	Util_ps()
	Util_runcommand(cmdline)
}

func Util_rm() {
	cmdline := "rm -rf data"
	fmt.Println(cmdline)
	Util_runcommand(cmdline)
}
