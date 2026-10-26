package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kevinadi/mlgo/internal/topology"
)

type ReplSet struct {
	Mongod      []*Mongod
	ReplSetName string
	Port        int
	Auth        bool
	Config      string
	Num         int
	Connstr     string
	Cmdlines    [][]string
	Script      bool
	Noinit      bool
}

func (rs *ReplSet) Init(num int, port int, config string, cfgSet bool, numSet bool, replsetname string, auth bool, noinit bool, script bool, role topology.ServerRole) error {
	plan, err := topology.PlanReplica(replsetname, port, config, num, cfgSet, numSet)
	if err != nil {
		return err
	}

	rs.Num = len(plan.Members)
	rs.ReplSetName = plan.Name
	rs.Port = plan.Port
	rs.Auth = auth
	rs.Config = plan.Config
	rs.Script = script
	rs.Noinit = noinit
	rs.Connstr = topology.ConnString(plan.Name, plan.Members)

	for _, member := range plan.Members {
		m_i := new(Mongod)
		if err := m_i.Init(member.Port, auth, replsetname, false, role); err != nil {
			return err
		}
		rs.Mongod = append(rs.Mongod, m_i)
	}

	rs.Cmdlines = rs.Cmd_mongod()
	if !rs.Noinit {
		init, err := rs.cmdInit(plan.Members)
		if err != nil {
			return err
		}
		rs.Cmdlines = append(rs.Cmdlines, init)
	}
	return nil
}

func (rs *ReplSet) Cmd_mongod() [][]string {
	var cmdline [][]string
	for i := 0; i < rs.Num; i++ {
		cmdline = append(cmdline, rs.Mongod[i].Cmd_mkdir())
		cmdline = append(cmdline, rs.Mongod[i].Cmd_mongod())
	}
	return cmdline
}

func (rs *ReplSet) cmdInit(members []topology.Member) ([]string, error) {
	docs := make([]string, 0, len(members))
	for _, member := range members {
		docs = append(docs, jsMember(member))
	}
	eval := fmt.Sprintf("rs.initiate({_id:%s, members:[%s]})", jsString(rs.ReplSetName), strings.Join(docs, ", "))
	return []string{"mongo", "--port", strconv.Itoa(rs.Port), "--eval", eval}, nil
}

// jsMember renders a member document as JavaScript with single-quoted
// strings, so the eval contains no double quotes. Util_cmd_script wraps the
// argument in double quotes, and cmd.exe toggles quoting on them.
func jsMember(m topology.Member) string {
	fields := []string{fmt.Sprintf("_id:%d", m.ID), "host:" + jsString(m.Host)}
	if m.Priority != nil {
		fields = append(fields, fmt.Sprintf("priority:%d", *m.Priority))
	}
	if m.Votes != nil {
		fields = append(fields, fmt.Sprintf("votes:%d", *m.Votes))
	}
	if m.Arbiter {
		fields = append(fields, "arbiterOnly:true")
	}
	if m.Hidden {
		fields = append(fields, "hidden:true")
	}
	return "{" + strings.Join(fields, ", ") + "}"
}

func jsString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", `\'`)
	return "'" + s + "'"
}

func (rs *ReplSet) Wait_for_primary() {
	var cmdline []string
	cmdline = append(cmdline, []string{
		"mongo",
		"--port", strconv.Itoa(rs.Port),
		"--quiet",
		"--eval", "db.isMaster()",
	}...)

	fmt.Println("Waiting for primary...")
	primary := false
	i := 0
	for !primary && i < 20 {
		com := exec.Command(cmdline[0], cmdline[1:]...)
		out, err := com.CombinedOutput()
		if err != nil {
			log.Fatalf("isMaster failed with %s\n", err)
		}
		primary = strings.Contains(string(out), "\"ismaster\" : true")
		time.Sleep(2 * time.Second)
		i += 1
	}

	if !primary {
		fmt.Println("Primary not found after 40 seconds")
		os.Exit(1)
	}
}

func (rs *ReplSet) Create_keyfile() {
	Util_runcommand_string([]string{"mkdir", Datadir})

	outfile, err := os.Create(fmt.Sprintf("%s/keyfile.txt", Datadir))
	if err != nil {
		log.Fatal(err)
	}
	defer outfile.Close()
	Check(err)

	_, err = outfile.WriteString(Util_randstring(40))
	Check(err)

	if runtime.GOOS != "windows" {
		err = outfile.Chmod(0600)
		Check(err)
	}
}

func (rs *ReplSet) Cmd_adduser() {
	Util_runcommand_string(rs.Mongod[0].Cmd_adduser())
}

func (rs *ReplSet) Deploy() {
	fmt.Println("# Port:", rs.Port)
	fmt.Println("# Num nodes:", rs.Num)
	fmt.Println("# Configuration:", rs.Config)
	fmt.Println("# Auth:", rs.Auth)
	fmt.Println("# Connection string:", rs.Connstr)
	fmt.Println()

	if rs.Script {
		fmt.Println(Util_cmd_script(rs.Cmdlines))
	} else {
		if rs.Auth {
			rs.Create_keyfile()
		}
		Util_runcommand_string_string(rs.Cmdlines)
		if !rs.Noinit {
			rs.Wait_for_primary()
		}
		if rs.Auth {
			rs.Cmd_adduser()
		}
		Util_create_start_script(rs.Cmdlines)
		fmt.Println(" -- ", "connstr :", rs.Connstr)
	}
}
