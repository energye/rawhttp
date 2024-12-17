package rawhttp

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

func TestGetSource(t *testing.T) {
	url := "https://sourceforge.net/projects/liblcl/files/v2.3.8/liblcl-109.Windows64.zip"
	client := NewClient(DefaultOptions)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bufRead(resp, "liblcl.Windows64.zip")
}

func TestGetTxtSource(t *testing.T) {
	url := "https://sourceforge.net/projects/liblcl/files/v2.3.7/md5.txt"
	client := NewClient(DefaultOptions)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bufRead(resp, "md5.txt")
}

func _TestProxyGetSource(t *testing.T) {
	url := "https://sourceforge.net/projects/liblcl/files/v2.3.8/liblcl-109.Windows64.zip"
	options := *DefaultOptions
	options.Proxy = "http://127.0.0.1:10809"
	client := NewClient(&options)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bufRead(resp, "liblcl-109.Windows64.zip")
}

func bufRead(resp *http.Response, saveName string) {
	if resp.Body == nil {
		return
	}
	file, err := os.Create(saveName)
	if err != nil {
		return
	}
	defer file.Close()
	read := bufio.NewReader(resp.Body)
	var (
		fSize   = resp.ContentLength // strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 32)
		buf     = make([]byte, 1024*10)
		written int64
		nw      int
	)
	var callback = func(totalLength, processLength int64) {
		fmt.Println("totalLength:", totalLength, "processLength:", processLength)
	}
	for {
		nr, er := read.Read(buf)
		if nr > 0 {
			nw, err = file.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			callback(fSize, written)
			if err != nil || nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
}
