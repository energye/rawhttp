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
	"time"
)

// Client is a client for making raw http requests with go
type Client struct {
	dialer  Dialer
	Options *Options
	jar     *cookiejar.Jar
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
	jar, _ := cookiejar.New(nil)
	client := &Client{
		dialer:  new(dialer),
		Options: options,
		jar:     jar,
	}
	return client
}

// Head makes a HEAD request to a given URL
func (c *Client) Head(url string) (*http.Response, error) {
	return c.DoRaw("HEAD", url, "", nil, nil)
}

// Get makes a GET request to a given URL
func (c *Client) Get(url string) (*http.Response, error) {
	return c.DoRaw("GET", url, "", nil, nil)
}

// Post makes a POST request to a given URL
func (c *Client) Post(url string, mimetype string, body io.Reader) (*http.Response, error) {
	headers := make(map[string][]string)
	headers["Content-Type"] = []string{mimetype}
	return c.DoRaw("POST", url, "", headers, body)
}

// Do sends a http request and returns a response
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	method := req.Method
	headers := req.Header
	url := req.URL.String()
	body := req.Body
	return c.DoRaw(method, url, "", headers, body)
}

// DoRaw does a raw request with some configuration
func (c *Client) DoRaw(method, url, uripath string, headers map[string][]string, body io.Reader) (*http.Response, error) {
	rs := &RedirectStatus{
		FollowRedirects: true,
		MaxRedirects:    c.Options.MaxRedirects,
	}
	return c.send(method, url, headers, body, rs, c.Options)
}

// DoRawWithOptions performs a raw request with additional options
func (c *Client) DoRawWithOptions(method, url, uripath string, headers map[string][]string, body io.Reader, options *Options) (*http.Response, error) {
	rs := &RedirectStatus{
		FollowRedirects: options.FollowRedirects,
		MaxRedirects:    c.Options.MaxRedirects,
	}
	return c.do(method, url, uripath, headers, body, rs, options)
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
	var proxyURL *url.URL
	if options.Proxy != "" {
		var err error
		proxyURL, err = url.Parse(options.Proxy)
		if err != nil {
			return nil, err
		}
	}
	httpClient := &http.Client{
		Jar: c.jar,
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
			Proxy: http.ProxyURL(proxyURL),
		},
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

func (c *Client) do(method, uri, uriPath string, headers map[string][]string, body io.Reader, redirectStatus *RedirectStatus, options *Options) (*http.Response, error) {
	protocol := "http"
	if strings.HasPrefix(strings.ToLower(uri), "https://") {
		protocol = "https"
	}
	if headers == nil {
		headers = make(map[string][]string)
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if options.AutomaticHostHeader {
		// add automatic space
		headers["Host"] = []string{fmt.Sprintf(" %s", host)}
	}

	if !strings.Contains(host, ":") {
		if protocol == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// standard path
	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	// override if custom one is specified
	if uriPath != "" {
		path = uriPath
	}
	if strings.HasPrefix(uri, "https://") {
		protocol = "https"
	}

	conn, err := c.getConn(protocol, host, options)
	if err != nil {
		return nil, err
	}

	req := toRequest(method, path, nil, headers, body, options)
	req.AutomaticContentLength = options.AutomaticContentLength
	req.AutomaticHost = options.AutomaticHostHeader

	// set timeout if any
	if options.Timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(options.Timeout))
	}

	if err := conn.WriteRequest(req); err != nil {
		return nil, err
	}
	resp, err := conn.ReadResponse(options.ForceReadAllBody)
	if err != nil {
		return nil, err
	}

	r, err := toHTTPResponse(conn, resp)
	if err != nil {
		return nil, err
	}

	if resp.Status.IsRedirect() && redirectStatus.FollowRedirects && redirectStatus.Current <= redirectStatus.MaxRedirects {
		// consume the response body
		_, err := io.Copy(io.Discard, r.Body)
		if err := firstErr(err, r.Body.Close()); err != nil {
			return nil, err
		}
		loc := headerValue(r.Header, "Location")
		if strings.HasPrefix(loc, "/") {
			loc = fmt.Sprintf("%s://%s%s", protocol, host, loc)
		}
		redirectStatus.Current++
		return c.do(method, loc, uriPath, headers, body, redirectStatus, options)
	}

	return r, err
}

// RedirectStatus is the current redirect status for the request
type RedirectStatus struct {
	FollowRedirects bool
	MaxRedirects    int
	Current         int
}
