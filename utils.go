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
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
)

type MongoProcess struct {
	Cmdline string
	Dbpath  string
	Port    string
	Pid     string
	Replset string
	Wd      string
}

func (p *MongoProcess) Init(cmdline string, pid string) {
	p.Cmdline = cmdline
	p.Pid = pid
	p.Get_dbpath()
	p.Get_port()
	p.Get_replset()
	p.Wd = Util_guess_dbpath(cmdline)
}

func (p *MongoProcess) String() string {
	return strings.Join([]string{p.Pid, p.Cmdline}, " ")
}

func (p *MongoProcess) Get_dbpath() {
	re := regexp.MustCompile("--(?:db|log)path ([^\\s]+)")
	if match := re.FindStringSubmatch(p.Cmdline); len(match) > 0 {
		p.Dbpath = match[1]
	}
}

func (p *MongoProcess) Get_port() {
	re := regexp.MustCompile("--port ([0-9]+)")
	if match := re.FindStringSubmatch(p.Cmdline); len(match) > 0 {
		p.Port = match[1]
	}
}

func (p *MongoProcess) Get_replset() {
	re := regexp.MustCompile("--replSet ([^\\s]+)")
	if match := re.FindStringSubmatch(p.Cmdline); len(match) > 0 {
		p.Replset = match[1]
	}
}

type MongoProcesses []*MongoProcess

func (pp MongoProcesses) String() string {
	var output []string
	for _, p := range pp {
		output = append(output, p.String())
	}
	return strings.Join(output, "\n")
}

func (pp MongoProcesses) Pretty() string {
	var output []string
	var workdir string
	currdir, _ := os.Getwd()

	if len(pp) == 0 {
		return "No running processes"
	}

	for _, wd := range pp.Workdirs() {
		workdir = wd
		if wd == currdir {
			workdir += " (current directory)"
		}

		output = append(output, workdir)
		for _, p := range pp {
			if p.Wd == wd {
				output = append(output, p.String())
			}
		}
		output = append(output, "")
	}

	return strings.Join(output, "\n")
}

func (pp MongoProcesses) Workdirs() []string {
	var output []string
	workdirs := make(map[string]struct{})
	for _, p := range pp {
		workdirs[p.Wd] = struct{}{}
	}
	for k, _ := range workdirs {
		output = append(output, k)
	}
	return output
}

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func Procs_get() MongoProcesses {
	procs, _ := process.Processes()
	var mongo_procs []*MongoProcess
	for _, proc := range procs {
		if p_name, _ := proc.Name(); strings.Contains(p_name, "mongod") || strings.Contains(p_name, "mongos") {
			p_cmd, _ := proc.Cmdline()
			mongo_proc := new(MongoProcess)
			mongo_proc.Init(p_cmd, strconv.Itoa(int(proc.Pid)))
			mongo_procs = append(mongo_procs, mongo_proc)
		}
	}
	return mongo_procs
}

func Util_randstring(num int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randstr := make([]byte, num)
	for i := range randstr {
		randstr[i] = letters[rand.Intn(len(letters))]
	}
	return string(randstr)
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

func Util_guess_dbpath(line string) string {
	var output string
	regexstring := fmt.Sprintf("--(?:db|log)path (.+)/%s/[0-9]+", Datadir)
	dbpath := regexp.MustCompile(regexstring)
	matches := dbpath.FindStringSubmatch(line)
	if len(matches) > 1 {
		output = matches[1]
	}
	return output
}

func Util_ps(what string) string {
	var output string
	var outputarr []string
	mongo_procs := Procs_get()
	if what != "" {
		for _, p := range mongo_procs {
			if strings.Contains(p.String(), what) {
				outputarr = append(outputarr, p.String())
			}
		}
		output = strings.Join(outputarr, "\n")
	} else {
		output = mongo_procs.String()
	}
	return output
}

func Util_ps_pretty() string {
	return Procs_get().Pretty()
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
