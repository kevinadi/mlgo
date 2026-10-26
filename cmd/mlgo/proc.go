package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/kevinadi/mlgo/internal/topology"
	"github.com/shirou/gopsutil/v4/process"
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
	p.Get_workpath()
}

func (p *MongoProcess) String() string {
	return strings.Join([]string{p.Pid, p.Cmdline}, " ")
}

func (p *MongoProcess) Get_dbpath() {
	re := regexp.MustCompile(`--(?:db|log)path ([^\s]+)`)
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
	re := regexp.MustCompile(`--replSet ([^\s]+)`)
	if match := re.FindStringSubmatch(p.Cmdline); len(match) > 0 {
		p.Replset = match[1]
	}
}

func (p *MongoProcess) Get_workpath() {
	// mongocmd builds these paths with filepath.Join, so Windows uses a backslash.
	regexstring := fmt.Sprintf(`--(?:db|log)path (.+)[/\\]%s[/\\][0-9]+`, regexp.QuoteMeta(topology.DataDir))
	re := regexp.MustCompile(regexstring)
	if match := re.FindStringSubmatch(p.Cmdline); len(match) > 0 {
		p.Wd = match[1]
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
	currdir, _ := os.Getwd()
	if len(pp) == 0 {
		return "No running processes"
	}
	for _, wd := range pp.Workdirs() {
		workdir := wd
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
	for k := range workdirs {
		output = append(output, k)
	}
	return output
}

func Procs_get() MongoProcesses {
	procs, _ := process.Processes()
	var mongoProcs []*MongoProcess
	for _, proc := range procs {
		name, _ := proc.Name()
		if strings.Contains(name, "mongod") || strings.Contains(name, "mongos") {
			cmdline, _ := proc.Cmdline()
			mongoProc := new(MongoProcess)
			mongoProc.Init(cmdline, strconv.Itoa(int(proc.Pid)))
			mongoProcs = append(mongoProcs, mongoProc)
		}
	}
	return mongoProcs
}

func Util_runcommand(cmdline string) string {
	var com *exec.Cmd
	if runtime.GOOS != "windows" {
		com = exec.Command("sh", "-c", cmdline)
	} else {
		com = exec.Command("cmd", "/c", cmdline)
	}
	stdout, err := com.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := com.Start(); err != nil {
		log.Fatal(err)
	}
	output, _ := io.ReadAll(stdout)
	return strings.TrimSpace(string(output))
}

func Util_ps(what string) string {
	mongoProcs := Procs_get()
	if what == "" {
		return mongoProcs.String()
	}
	var output []string
	for _, p := range mongoProcs {
		if strings.Contains(p.String(), what) {
			output = append(output, p.String())
		}
	}
	return strings.Join(output, "\n")
}

func Util_ps_pretty() string {
	return Procs_get().Pretty()
}

func Util_kill(what string) {
	var cmdline string
	var psOutput string
	if runtime.GOOS != "windows" {
		if what == "" {
			pwd, _ := os.Getwd()
			psOutput = Util_ps(fmt.Sprintf("%s/%s/", pwd, topology.DataDir))
			if psOutput == "" {
				fmt.Println("No processes running under current directory")
				os.Exit(1)
			}
		} else if what == "all" {
			psOutput = Util_ps("")
		} else {
			psOutput = Util_ps(what)
		}
		var pids []string
		for _, line := range strings.Split(psOutput, "\n") {
			pids = append(pids, strings.Split(line, " ")[0])
		}
		cmdline = fmt.Sprintf("kill %s", strings.Join(pids, " "))
	} else {
		cmdline = "taskkill /f /im mongod.exe & taskkill /f /im mongos.exe"
	}
	fmt.Println("Killing processes...")
	fmt.Println(psOutput)
	Util_runcommand(cmdline)
}

func Util_rm() {
	var cmdline string
	if runtime.GOOS != "windows" {
		cmdline = fmt.Sprintf("rm -rf %s", topology.DataDir)
	} else {
		cmdline = fmt.Sprintf("rmdir /q /s %s", topology.DataDir)
	}
	fmt.Println(cmdline)
	Util_runcommand(cmdline)
}

func Util_start_process(what string) {
	var cmdline string
	if what != "" {
		cmdline = fmt.Sprintf("cat %s/start.sh | grep %s | sh", topology.DataDir, what)
		fmt.Println("Starting", what, "...")
	} else {
		cmdline = fmt.Sprintf("cat %s/start.sh | sh", topology.DataDir)
		fmt.Println("Starting all processes ...")
	}
	fmt.Println(Util_runcommand(cmdline))
}

func Util_install_m(where string) string {
	cmdline := "curl https://raw.githubusercontent.com/aheckmann/m/master/bin/m"
	if where != "" {
		cmdline += fmt.Sprintf(" > %s/m && chmod 755 %s/m", where, where)
	} else {
		cmdline += " > /usr/local/bin/m && chmod 755 /usr/local/bin/m"
	}
	return cmdline
}
