package main

import "testing"

func TestWorkpathAcceptsBothSeparators(t *testing.T) {
	for cmdline, want := range map[string]string{
		`mongod --dbpath /work/mlgo-data/27017 --port 27017`:       "/work",
		`mongod.exe --dbpath C:\work\mlgo-data\27017 --port 27017`: `C:\work`,
	} {
		p := new(MongoProcess)
		p.Init(cmdline, "1")
		if p.Wd != want {
			t.Errorf("%s: Wd = %q, want %q", cmdline, p.Wd, want)
		}
	}
}
