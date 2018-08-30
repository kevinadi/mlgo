package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os/exec"
)

func Util_create_keyfile() string {
	var cmdline string
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 40)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	cmdline = fmt.Sprintf("echo %s > data/keyfile.txt && chmod 600 data/keyfile.txt", string(b))
	return cmdline + "\n"
}

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

func Util_create_dbpath() string {
	var cmdline string
	cmdline = "mkdir data"
	return cmdline + "\n"
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

func Util_create_first_user(port int) string {
	user := "{user: 'user', pwd: 'password', roles: ['root']}"
	cmdline := fmt.Sprintf("mongo --host localhost --port %d admin ", port)
	cmdline += fmt.Sprintf("--eval \"db.createUser(%s)\"", user)
	return cmdline + "\n"
}
