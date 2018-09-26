package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func Util_randstring(num int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randstr := make([]byte, num)
	for i := range randstr {
		randstr[i] = letters[rand.Intn(len(letters))]
	}
	return string(randstr)
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
			if strings.Contains(cmd, " ") ||
				strings.Contains(cmd, "(") {
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

func Util_findstr(hay []string, needle string) bool {
	for _, l := range hay {
		if l == needle {
			return true
		}
	}
	return false
}

func Util_runcommand_string(line []string) {
	var com *exec.Cmd
	var err error

	fmt.Println(">>>", line)
	if runtime.GOOS == "windows" {
		line = append([]string{"cmd", "/c"}, line...)
	}

	for i := 0; i < 5; i++ {
		com = exec.Command(line[0], line[1:]...)
		com.Stdout = os.Stdout
		com.Stderr = os.Stderr

		com.Start()
		err = com.Wait()
		if err != nil {
			if Util_findstr(line, "mongo") {
				fmt.Println("Retrying command...")
				time.Sleep(2 * time.Second)
			} else {
				fmt.Println("Command", strings.Join(line, " "), "failed with", err)
				os.Exit(1)
			}
		} else {
			break
		}
	}
}

func Util_runcommand_string_string(cmdlines [][]string) {
	for _, line := range cmdlines {
		Util_runcommand_string(line)
	}
}

func Util_create_start_script(cmdlines [][]string) {
	outfile, err := os.Create(fmt.Sprintf("%s/start.sh", Datadir))
	defer outfile.Close()
	Check(err)

	script := Util_cmd_script(cmdlines)
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "mongod") ||
			strings.Contains(line, "mongos") {
			_, err := outfile.WriteString(line + "\n")
			Check(err)
		}
	}
	outfile.Sync()
}

func Util_runcommand(cmdline string) string {
	var com *exec.Cmd
	if runtime.GOOS != "windows" {
		com = exec.Command("sh", "-c", cmdline)
	} else {
		com = exec.Command("cmd", "/c", cmdline)
	}
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
	cmdline = fmt.Sprintf("mkdir %s", Datadir)
	return cmdline
}

func Util_guess_dbpath(line string) string {
	var output string
	regexstring := fmt.Sprintf("--dbpath ([^ ]+)/%s/[0-9]+ ", Datadir)
	dbpath := regexp.MustCompile(regexstring)
	matches := dbpath.FindStringSubmatch(line)
	if len(matches) > 1 {
		output = matches[1]
	}
	return output
}

func Util_list_dbpath(lines []string) []string {
	var output []string
	var pathlist = map[string]bool{}
	for _, line := range lines {
		pathlist[Util_guess_dbpath(line)] = true
	}
	for k, _ := range pathlist {
		output = append(output, k)
	}
	return output
}

func Util_list_all_dbpath(ps string) string {
	var output []string
	pslist := strings.Split(ps, "\n")
	dbpaths := Util_list_dbpath(pslist)
	pwd, _ := os.Getwd()

	for _, path := range dbpaths {
		if path == pwd {
			output = append(output, fmt.Sprintf("Running processes under %s (current directory)", path))
		} else {
			output = append(output, fmt.Sprintf("Running processes under %s", path))
		}
		for _, cmd := range pslist {
			if strings.Contains(cmd, path) {
				output = append(output, cmd)
			}
		}
		output = append(output, "")
	}
	return strings.Join(output, "\n")
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
	var ps_output string
	var pids []string

	if runtime.GOOS != "windows" {
		if what == "" {
			pwd, _ := os.Getwd()
			dbpath_pwd := fmt.Sprintf("%s/%s/", pwd, Datadir)

			ps_output = Util_ps(dbpath_pwd)

			if ps_output == "" {
				fmt.Println("No processes running under current directory")
				os.Exit(1)
			}
		} else if what == "all" {
			ps_output = Util_ps("")
		} else {
			ps_output = Util_ps(what)
		}

		for _, m := range strings.Split(ps_output, "\n") {
			pids = append(pids, strings.Split(m, " ")[0])
		}
		cmdline = fmt.Sprintf("kill %s", strings.Join(pids, " "))
	} else {
		cmdline = "taskkill /f /im mongod.exe & taskkill /f /im mongos.exe"
	}

	fmt.Println("Killing processes...")
	fmt.Println(ps_output)
	Util_runcommand(cmdline)
}

func Util_rm() {
	var cmdline string
	if runtime.GOOS != "windows" {
		cmdline = fmt.Sprintf("rm -rf %s", Datadir)
	} else {
		cmdline = fmt.Sprintf("rmdir /q /s %s", Datadir)
	}
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
	cmdline += fmt.Sprintf("cat << EOF > %s/start.sh\\n", Datadir)
	cmdline += script + "\n"
	cmdline += "EOF"
	return cmdline
}

func Util_start_process(what string) {
	var cmdline string
	if what != "" {
		cmdline = fmt.Sprintf("cat %s/start.sh | grep %s | sh", Datadir, what)
		fmt.Println("Starting", what, "...")
	} else {
		cmdline = fmt.Sprintf("cat %s/start.sh | sh", Datadir)
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
