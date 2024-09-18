package rawhttp

import (
	"io/ioutil"
	"net/http/httputil"
	"testing"
)

func TestGet(t *testing.T) {
	url := "https://sourceforge.net/projects/liblcl/files/v2.3.7/liblcl-49.WindowsXP_SP3_64.zip"
	client := NewClient(DefaultOptions)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	bin, err := httputil.DumpResponse(resp, true)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile("liblcl-49.WindowsXP_SP3_64.zip", bin, 0644)
}
