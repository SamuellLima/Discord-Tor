package portkill

import (
	"reflect"
	"testing"
)

func TestListeningPIDsFromNetstat(t *testing.T) {
	const sample = `
  Proto  Local Address          Foreign Address        State           PID
  TCP    127.0.0.1:9060         0.0.0.0:0              LISTENING       4242
  TCP    127.0.0.1:19060        0.0.0.0:0              LISTENING       7777
  TCP    0.0.0.0:9060           0.0.0.0:0              LISTENING       4242
  TCP    [::1]:9060             [::]:0                 LISTENING       4243
  TCP    ::1:9060               ::0                    LISTENING       4244
  TCP    127.0.0.1:9060         127.0.0.1:51234        ESTABLISHED     9999
  UDP    127.0.0.1:9060         *:*                                    1111
`
	got := ListeningPIDsFromNetstat(sample, "9060")
	want := []string{"4242", "4243", "4244"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := ListeningPIDsFromNetstat(sample, "19060"); !reflect.DeepEqual(got, []string{"7777"}) {
		t.Fatalf("19060: got %v", got)
	}
	if got := ListeningPIDsFromNetstat(sample, "80"); len(got) != 0 {
		t.Fatalf("empty port: got %v", got)
	}
}
