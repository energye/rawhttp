package rawhttp

import (
	"fmt"
	"net/http/httputil"
	"testing"
)

func TestGet(t *testing.T) {
	url := "http://scanme.sh"
	client := NewClient(DefaultOptions)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	bin, err := httputil.DumpResponse(resp, true)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bin))
}
