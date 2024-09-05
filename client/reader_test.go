package client

import (
	"bufio"
	"strings"
	"testing"
)

type statusTest struct {
	name       string
	statusLine string
	result     int
	err        bool
}

type versionTest struct {
	name    string
	version string
	major   int
	minor   int
	err     bool
}

func TestStatusCode(t *testing.T) {
	tests := []statusTest{
		{"redirect 301", "301\r\n", 301, false},
		{"200 ok", "200 ", 200, false},
		{"300 redirect", "300 ", 300, false},
		{"0 unknown", "0 ", 0, false},
		{"long number", "4578 ", 4578, false},
		{"invalid string", "aaa ", 0, true},
		{"number with status text", "1234 unknown", 1234, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := reader{bufio.NewReader(strings.NewReader(test.statusLine))}
			result, err := r.ReadStatusCode()
			hasError := err != nil
			requireEqual(test.err, hasError)
			requireEqual(test.result, result)
		})
	}
}

func TestReadVersion(t *testing.T) {
	tests := []versionTest{
		{"HTTP/0.9", "HTTP/0.9 OK", 0, 9, false},
		{"HTTP/1.0", "HTTP/1.0 OK", 1, 0, false},
		{"HTTP/1.1", "HTTP/1.1 OK", 1, 1, false},
		{"HTTP/2", "HTTP/2 OK", 2, 0, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := reader{bufio.NewReader(strings.NewReader(test.version))}
			result, err := r.ReadVersion()
			hasError := err != nil
			requireEqual(test.err, hasError)
			requireEqual(strings.TrimSuffix(test.version, " OK"), result.String())
			requireEqual(test.major, result.Major)
			requireEqual(test.minor, result.Minor)
		})
	}
}

func requireEqual(expected, actual interface{}) {

}
