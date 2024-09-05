package rawhttp

import (
	"crypto/tls"
	"github.com/energye/rawhttp/client"
	"io"
	"net"
	"sync"
	"time"
)

// Dialer can dial a remote HTTP server.
type Dialer interface {
	// Dial dials a remote http server returning a Conn.
	Dial(protocol, addr string, options *Options) (Conn, error)
	// DialTimeout dials a remote http server with timeout returning a Conn.
	DialTimeout(protocol, addr string, timeout time.Duration, options *Options) (Conn, error)
}

type dialer struct {
	sync.Mutex                   // protects following fields
	conns      map[string][]Conn // maps addr to a, possibly empty, slice of existing Conns
}

func (d *dialer) Dial(protocol, addr string, options *Options) (Conn, error) {
	return d.dialTimeout(protocol, addr, 0, options)
}

func (d *dialer) DialTimeout(protocol, addr string, timeout time.Duration, options *Options) (Conn, error) {
	return d.dialTimeout(protocol, addr, timeout, options)
}

func (d *dialer) dialTimeout(protocol, addr string, timeout time.Duration, options *Options) (Conn, error) {
	d.Lock()
	if d.conns == nil {
		d.conns = make(map[string][]Conn)
	}
	if c, ok := d.conns[addr]; ok {
		if len(c) > 0 {
			conn := c[0]
			c[0] = c[len(c)-1]
			d.Unlock()
			return conn, nil
		}
	}
	d.Unlock()
	c, err := clientDial(protocol, addr, timeout, options)
	return &conn{
		Client: client.NewClient(c),
		Conn:   c,
		dialer: d,
	}, err
}

func clientDial(protocol, addr string, timeout time.Duration, options *Options) (net.Conn, error) {
	// http
	if protocol == "http" {
		if timeout > 0 {
			return net.DialTimeout("tcp", addr, timeout)
		}
		return net.Dial("tcp", addr)
	} else {
		// https
		tlsConfig := &tls.Config{InsecureSkipVerify: true, Renegotiation: tls.RenegotiateOnceAsClient}
		if options.SNI != "" {
			tlsConfig.ServerName = options.SNI
		}
		var dialer *net.Dialer
		if timeout > 0 {
			dialer = &net.Dialer{Timeout: timeout}
		} else {
			dialer = &net.Dialer{Timeout: 8 * time.Second} // should be more than enough
		}
		return tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	}
}

// Conn is an interface implemented by a connection
type Conn interface {
	client.Client
	io.Closer
	SetDeadline(time.Time) error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	Release()
}

type conn struct {
	client.Client
	net.Conn
	*dialer
}

func (c *conn) Release() {
	c.dialer.Lock()
	defer c.dialer.Unlock()
	addr := c.Conn.RemoteAddr().String()
	c.dialer.conns[addr] = append(c.dialer.conns[addr], c)
}
