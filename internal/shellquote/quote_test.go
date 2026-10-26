package shellquote

import "testing"

func TestQuote(t *testing.T) {
	if got := Quote("mongod"); got != "mongod" {
		t.Fatalf("got %s", got)
	}
	if got := Quote(""); got != "''" {
		t.Fatalf("got %s", got)
	}
	got := Quote(`rs.initiate({"_id":"replset"})`)
	if got != `'rs.initiate({"_id":"replset"})'` {
		t.Fatalf("got %s", got)
	}
	if got := Quote("a b"); got != "'a b'" {
		t.Fatalf("got %s", got)
	}
}
