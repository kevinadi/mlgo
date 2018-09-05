package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func Util_create_keyfile() string {
	var cmdline string
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 40)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	cmdline = fmt.Sprintf("echo %s > data/keyfile.txt && chmod 600 data/keyfile.txt", string(b))
	return cmdline
}

func Util_cmd_script(cmd_array [][]string) string {
	var script string
	for _, line := range cmd_array {
		for _, cmd := range line {
			if strings.Contains(cmd, " ") {
				script += fmt.Sprintf("\"%s\"", cmd)
			} else {
				script += fmt.Sprintf("%s", cmd)
			}
			script += fmt.Sprintf(" ")
		}
		script += fmt.Sprintf("\n")
	}
	return script
}

func Util_runcommand_string(cmdlines []string) {
	for _, line := range cmdlines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fmt.Println("\n>>>", line)
		ll := strings.Fields(line)
		com := exec.Command(ll[0], ll[1:]...)
		com.Stdout = os.Stdout
		com.Start()
		com.Wait()
	}
}

func Util_runcommand_string_string(cmdlines [][]string) {
	var com *exec.Cmd
	for _, line := range cmdlines {
		fmt.Println("\n>>>", line)
		if runtime.GOOS == "windows" {
			line = append([]string{"cmd", "/c"}, line...)
		}
		com = exec.Command(line[0], line[1:]...)
		com.Stdout = os.Stdout
		com.Stderr = os.Stderr
		com.Start()
		com.Wait()
	}
}

func Util_create_start_script(cmdlines [][]string) {
	outfile, err := os.Create("data/start.sh")
	defer outfile.Close()
	check(err)

	script := Util_cmd_script(cmdlines)
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "mongod") ||
			strings.Contains(line, "mongos") {
			_, err := outfile.WriteString(line)
			check(err)
		}
	}
	outfile.Sync()
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

func Util_runcommand_stdout(cmdline string) {
	com := exec.Command("sh", "-c", cmdline)
	com.Stdout = os.Stdout
	if err := com.Start(); err != nil {
		log.Fatal(err)
	}
	com.Wait()
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

func Util_install_m(where string) string {
	var cmdline string = "curl https://raw.githubusercontent.com/aheckmann/m/master/bin/m"
	if where != "" {
		cmdline += fmt.Sprintf(" > %s/m && chmod 755 %s/m", where, where)
	} else {
		cmdline += " > /usr/local/bin/m && chmod 755 /usr/local/bin/m"
	}
	return cmdline
}
