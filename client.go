package rawhttp

import (
	"errors"
	"fmt"
	"github.com/energye/rawhttp/client"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// Client is a client for making raw http requests with go
type Client struct {
	dialer   Dialer
	Options  *Options
	Jar      *cookiejar.Jar
	proxyURL *url.URL
}

// AutomaticHostHeader sets Host header for requests automatically
func AutomaticHostHeader(enable bool) {
	DefaultClient.Options.AutomaticHostHeader = enable
}

// AutomaticContentLength performs automatic calculation of request content length.
func AutomaticContentLength(enable bool) {
	DefaultClient.Options.AutomaticContentLength = enable
}

// NewClient creates a new rawhttp client with provided options
func NewClient(options *Options) *Client {
	client := &Client{
		dialer:  new(dialer),
		Options: options,
	}
	return client
}

// Head makes a HEAD request to a given URL
func (c *Client) Head(url string) (*http.Response, error) {
	return c.DoRaw("HEAD", url, nil, nil)
}

// Get makes a GET request to a given URL
func (c *Client) Get(url string) (*http.Response, error) {
	return c.DoRaw("GET", url, nil, nil)
}

// Post makes a POST request to a given URL
func (c *Client) Post(url string, mimetype string, body io.Reader) (*http.Response, error) {
	headers := make(map[string][]string)
	headers["Content-Type"] = []string{mimetype}
	return c.DoRaw("POST", url, headers, body)
}

// Do sends a http request and returns a response
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	method := req.Method
	headers := req.Header
	url := req.URL.String()
	body := req.Body
	return c.DoRaw(method, url, headers, body)
}

// DoRaw does a raw request with some configuration
func (c *Client) DoRaw(method, url string, headers map[string][]string, body io.Reader) (*http.Response, error) {
	rs := &RedirectStatus{
		FollowRedirects: true,
		MaxRedirects:    c.Options.MaxRedirects,
	}
	return c.send(method, url, headers, body, rs, c.Options)
}

// DoRawWithOptions performs a raw request with additional options
func (c *Client) DoRawWithOptions(method, url string, headers map[string][]string, body io.Reader, options *Options) (*http.Response, error) {
	rs := &RedirectStatus{
		FollowRedirects: options.FollowRedirects,
		MaxRedirects:    c.Options.MaxRedirects,
	}
	return c.send(method, url, headers, body, rs, options)
}

func (c *Client) getConn(protocol, host string, options *Options) (Conn, error) {
	if options.Proxy != "" {
		return c.dialer.DialWithProxy(protocol, host, c.Options.Proxy, c.Options.ProxyDialTimeout)
	}
	var conn Conn
	var err error
	if options.Timeout > 0 {
		conn, err = c.dialer.DialTimeout(protocol, host, options.Timeout, options)
	} else {
		conn, err = c.dialer.Dial(protocol, host, options)
	}
	return conn, err
}

func (c *Client) send(method, uri string, headers map[string][]string, body io.Reader, redirectStatus *RedirectStatus, options *Options) (*http.Response, error) {
	protocol := "http"
	if strings.HasPrefix(strings.ToLower(uri), "https://") {
		protocol = "https"
	}
	if options.Proxy != "" {
		var err error
		c.proxyURL, err = url.Parse(options.Proxy)
		if err != nil {
			return nil, err
		}
	}
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > redirectStatus.MaxRedirects {
				return errors.New(fmt.Sprintf("已达到最大重定向数: %v", redirectStatus.MaxRedirects))
			}
			if req.Response != nil {
				redirectUrl := headerValue(req.Response.Header, "Location")
				if redirectUrl != "" {
					return http.ErrUseLastResponse
				}
			}
			return nil
		},
		Transport: &http.Transport{
			Proxy: http.ProxyURL(c.proxyURL),
		},
	}
	if c.Jar != nil {
		httpClient.Jar = c.Jar
	}
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}
	for key, val := range headers {
		req.Header.Set(key, val[0])
	}
	host := req.Host
	if options.AutomaticHostHeader {
		req.Header.Set("Host", fmt.Sprintf(" %s", host))
	}
	req.Header.Set("User-Agent", "energy raw http/1.0.0")
	req.Header.Set("Accept", "*/*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	code := resp.StatusCode
	var isRedirect = func() bool {
		if code == client.REDIRECTION_NOT_MODIFIED {
			return false
		}
		return code >= client.REDIRECTION_MULTIPLE_CHOICES && code < client.CLIENT_ERROR_BAD_REQUEST
	}
	// 301 302 303 307 308
	if isRedirect() && redirectStatus.FollowRedirects && redirectStatus.Current <= redirectStatus.MaxRedirects {
		// consume the response body
		_, err := io.Copy(io.Discard, resp.Body)
		if err := firstErr(err, resp.Body.Close()); err != nil {
			return nil, err
		}
		redirectUrl := headerValue(resp.Header, "Location")
		if strings.HasPrefix(redirectUrl, "/") {
			redirectUrl = fmt.Sprintf("%s://%s%s", protocol, host, redirectUrl)
		}
		redirectStatus.Current++
		return c.send(method, redirectUrl, headers, body, redirectStatus, options)
	}

	return resp, nil
}

// RedirectStatus is the current redirect status for the request
type RedirectStatus struct {
	FollowRedirects bool
	MaxRedirects    int
	Current         int
}
