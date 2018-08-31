package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os/exec"
	"strings"
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

func Util_runcommand(cmdline string) string {
	com := exec.Command("sh", "-c", cmdline)
	comStdout, err := com.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := com.Start(); err != nil {
		log.Fatal(err)
	}
	output, _ := ioutil.ReadAll(comStdout)
	return strings.TrimSpace(string(output))
}

func Util_create_dbpath() string {
	var cmdline string
	cmdline = "mkdir data"
	return cmdline
}

func Util_ps(what string) string {
	var output_lines []string
	cmdline := "ps -Ao 'pid,command' | grep -v 'grep .* mongod' | grep '\\smongo[ds]\\s'"
	if what != "" {
		cmdline += fmt.Sprintf(" | grep %s", what)
	}
	for _, m := range strings.Split(Util_runcommand(cmdline), "\n") {
		output_lines = append(output_lines, strings.TrimSpace(string(m)))
	}
	return strings.Join(output_lines, "\n")
}

func Util_kill(what string) {
	var cmdline string
	var pids []string

	ps_output := Util_ps(what)
	fmt.Println(ps_output)
	for _, m := range strings.Split(ps_output, "\n") {
		pids = append(pids, strings.Split(m, " ")[0])
	}
	cmdline = fmt.Sprintf("kill %s", strings.Join(pids, " "))

	fmt.Println(cmdline)
	fmt.Println(Util_runcommand(cmdline))
}

func Util_rm() {
	cmdline := "rm -rf data"
	fmt.Println(cmdline)
	fmt.Println(Util_runcommand(cmdline))
}

func Util_create_first_user(port int) string {
	user := "{user: 'user', pwd: 'password', roles: ['root']}"
	cmdline := fmt.Sprintf("mongo --host localhost --port %d admin ", port)
	cmdline += fmt.Sprintf("--eval \"db.createUser(%s)\"", user)
	return cmdline + "\n"
}

func Util_start_script(script string) string {
	cmdline := "########\n"
	cmdline += "cat << EOF > data/start.sh\n"
	cmdline += script + "\n"
	cmdline += "EOF"
	return cmdline
}

func Util_start_process(what string) {
	var cmdline string
	if what != "" {
		cmdline = fmt.Sprintf("cat data/start.sh | grep %s | sh", what)
		fmt.Println("Starting", what, "...")
	} else {
		cmdline = fmt.Sprintf("cat data/start.sh | sh")
		fmt.Println("Starting all processes ...")
	}
	fmt.Println(Util_runcommand(cmdline))
}
