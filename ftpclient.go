// Package ftpclient implements a FTP client as described in RFC 959.
package ftpclient

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	network = "tcp"
)

// FTP Status Code, defined in RFC 959
const (
	DataConnectionAlreadyOpen = 125
	FileStatusOK              = 150
	CommandOkay               = 200
	SystemStatus              = 211
	DirectoryStatus           = 212
	FileStatus                = 213
	ConnectionClosing         = 221
	SystemType                = 215
	ServiceReadyForNewUser    = 220
	ClosingDataConnection     = 226
	UserLoggedIn              = 230
	ActionOK                  = 250
	PathCreated               = 257
	UserNameOK                = 331
	ActionPending             = 350
	NotLoggedIn               = 530
)

// FtpServerConn represents the connection to a remote FTP server.
type FtpServerConn struct {
	*Config
	passive       bool
	textprotoConn *textproto.Conn
	conn          net.Conn
}

// FtpDataConn represent a data-connection
type FtpDataConn struct {
	conn net.Conn
	c    *FtpServerConn
}

var regexp227 = regexp.MustCompile("([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+)")
var regexp229 = regexp.MustCompile("\\|\\|\\|([0-9]+)\\|")

func init() {
	//regexp227, _ = regexp.Compile("([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+)")
	//regexp229, _ = regexp.Compile("\\|\\|\\|([0-9]+)\\|")
}

// New ...
func New(cfg *Config) *FtpServerConn {
	c := &FtpServerConn{
		Config:  cfg,
		passive: false,
	}
	return c
}

// Dial ...
func (c *FtpServerConn) Dial(addr string) error {
	return c.DialTimeout(addr, 0)
}

// DialTimeout ...
func (c *FtpServerConn) DialTimeout(addr string, timeout time.Duration) error {
	var conn net.Conn
	var err error

	if c.tlsConfig != nil && c.tlsImplicit == true {
		dialer := &net.Dialer{
			Timeout: timeout,
		}
		conn, err = tls.DialWithDialer(dialer, network, addr, c.tlsConfig)
	} else {
		conn, err = net.DialTimeout(network, addr, timeout)
	}
	if err != nil {
		return err
	}

	textprotoConn := textproto.NewConn(conn)
	c.textprotoConn = textprotoConn
	c.conn = conn
	_, _, err = c.getResponse(ServiceReadyForNewUser)
	if err != nil {
		return err
	}

	return nil
}

// Login as the given user.
func (c *FtpServerConn) Login(user, password string) error {

	if c.tlsConfig != nil && c.tlsImplicit == false {
		if err := c.Auth("TLS"); err != nil {
			return err
		}

		conn := tls.Client(c.conn, c.tlsConfig)
		textprotoConn := textproto.NewConn(conn)
		c.textprotoConn = textprotoConn
		c.conn = conn

		if err := c.Pbsz("0"); err != nil {
			return err
		}

		if err := c.Prot("P"); err != nil {
			return err
		}
	}

	code, message, err := c.SendCmd(-1, "USER %s", user)
	if err != nil {
		return err
	}

	if code == UserNameOK {
		_, _, err = c.SendCmd(UserLoggedIn, "PASS %s", password)
		if err != nil {
			return err
		}
		return nil
	}

	return errors.New(message)
}

// Type issues a TYPE FTP command
func (c *FtpServerConn) Type(param string) error {
	_, _, err := c.SendCmd(CommandOkay, "TYPE %s", param)
	return err
}

// Cwd issues a CWD FTP command, which changes the current directory to the specified path.
func (c *FtpServerConn) Cwd(path string) error {
	_, _, err := c.SendCmd(ActionOK, "CWD %s", path)
	return err
}

// Cdup issues a CDUP FTP command, which changes the current directory to the parent directory.
// This is similar to a call to ChangeDir with a path set to "..".
func (c *FtpServerConn) Cdup() error {
	_, _, err := c.SendCmd(ActionOK, "CDUP")
	return err
}

// Pwd issues a PWD FTP command, which Returns the path of the current directory.
func (c *FtpServerConn) Pwd() (string, error) {
	_, msg, err := c.SendCmd(257, "PWD")
	if err != nil {
		return "", err
	}

	return parse257(msg)
}

// Rename renames a file on the remote FTP server.
func (c *FtpServerConn) Rename(from, to string) error {
	_, _, err := c.SendCmd(350, "RNFR %s", from)
	if err != nil {
		return err
	}
	_, _, err = c.SendCmd(ActionOK, "RNTO %s", to)
	return err
}

// Delete issues a DELE FTP command to delete the specified file from the remote FTP server.
func (c *FtpServerConn) Delete(path string) error {
	code, msg, err := c.SendCmd(-1, "DELE %s", path)
	if err != nil {
		return err
	}
	if code != ActionOK && code != CommandOkay {
		return &textproto.Error{Code: code, Msg: msg}
	}
	return err
}

// Mkd issues a MKD FTP command to create the specified directory on the remote FTP server.
func (c *FtpServerConn) Mkd(path string) (string, error) {
	_, msg, err := c.SendCmd(257, "MKD %s", path)
	if err != nil {
		return "", err
	}

	return parse257(msg)
}

// Rmd issues a RMD FTP command to remove the specified directory from the remote FTP server.
func (c *FtpServerConn) Rmd(path string) error {
	_, _, err := c.SendCmd(ActionOK, "RMD %s", path)
	return err
}

// Noop has no effects and is usually used to prevent the remote FTP server to close the otherwise idle connection.
func (c *FtpServerConn) Noop() error {
	_, _, err := c.SendCmd(CommandOkay, "NOOP")
	return err
}

// Rest issues a REST FTP command.
func (c *FtpServerConn) Rest(offset uint64) error {
	_, _, err := c.SendCmd(350, "REST %d", offset)
	return err
}

// Rein issues a REIN FTP command to logout the current user. ftp server optional command.
func (c *FtpServerConn) Rein() error {
	_, _, err := c.SendCmd(220, "REIN")
	return err
}

// Abort a file transfer that is in progress.
func (c *FtpServerConn) Abort() error {
	code, msg, err := c.SendCmd(-1, "ABOR")
	if err != nil {
		return err
	}
	if code != 225 && code != 226 {
		return &textproto.Error{Code: code, Msg: msg}
	}
	return err
}

// Syst issues a SYST FTP command
func (c *FtpServerConn) Syst() (string, error) {
	_, msg, err := c.SendCmd(215, "SYST")
	if err != nil {
		return "", err
	}

	return msg, nil
}

// Quit issues a QUIT FTP command to properly close the connection from the remote FTP server.
func (c *FtpServerConn) Quit() error {
	c.SendCmd(-1, "QUIT")
	//return c.conn.Close()
	return c.textprotoConn.Close()
}

// Size Request the size of the file named filename on the server.
// On success, the size of the file is returned as an integer.
// ftp server extention command.
func (c *FtpServerConn) Size(filename string) (int, error) {
	_, msg, err := c.SendCmd(213, "SIZE %s", filename)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(msg))
}

// NlstRequest issues an NLST FTP command.
func (c *FtpServerConn) NlstRequest(args ...string) (io.ReadCloser, error) {
	cmd := append([]string{"NLST"}, args...)
	val := strings.Join(cmd, " ")

	conn, err := c.transferCmd(val)
	if err != nil {
		return nil, err
	}

	return &FtpDataConn{conn, c}, nil
}

// ListRequest issues a LIST FTP command.
func (c *FtpServerConn) ListRequest(args ...string) (io.ReadCloser, error) {
	cmd := append([]string{"LIST"}, args...)
	val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return nil, err
	}

	return &FtpDataConn{conn, c}, nil
}

// RetrRequest issues a RETR FTP command to fetch the specified file from the remote FTP server
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *FtpServerConn) RetrRequest(path string) (io.ReadCloser, error) {
	conn, err := c.transferCmd("RETR %s", path)
	if err != nil {
		return nil, err
	}
	return &FtpDataConn{conn, c}, nil
}

// StorRequest issues a STOR FTP command to store a file to the remote FTP server.
// The returned WriteCloser must be closed to cleanup the FTP data connection.
func (c *FtpServerConn) StorRequest(path string) (io.WriteCloser, error) {
	conn, err := c.transferCmd("STOR %s", path)
	if err != nil {
		return nil, err
	}
	return &FtpDataConn{conn, c}, nil
}

// TransferRequest issues a FTP command to fetch the specified file from the remote FTP server
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *FtpServerConn) TransferRequest(format string, args ...interface{}) (io.ReadCloser, error) {
	conn, err := c.transferCmd(format, args...)
	if err != nil {
		return nil, err
	}
	return &FtpDataConn{conn, c}, nil
}

// SetPasv sets the mode to passive or active for data transfers.
func (c *FtpServerConn) SetPasv(ispassive bool) {
	c.passive = ispassive
}

// Nlst issues an NLST FTP command.
func (c *FtpServerConn) Nlst(args ...string) (lines []string, err error) {
	cmd := append([]string{"NLST"}, args...)
	val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return
	}

	r := &FtpDataConn{conn, c}
	defer r.Close()

	lines, err = c.getLines(r)
	if err != nil {
		return
	}
	return
}

// List issues a LIST FTP command.
func (c *FtpServerConn) List(args ...string) (lines []string, err error) {
	cmd := append([]string{"LIST"}, args...)
	val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return
	}

	r := &FtpDataConn{conn, c}
	defer r.Close()

	lines, err = c.getLines(r)
	if err != nil {
		return
	}
	return
}

// Dir issues a LIST FTP command.
func (c *FtpServerConn) Dir(args ...string) (infos []os.FileInfo, err error) {
	cmd := append([]string{"LIST"}, args...)
	val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return
	}

	r := &FtpDataConn{conn, c}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		fileinfo, err := parse(line)
		if err == nil {
			infos = append(infos, fileinfo)
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return
}

// Retr issues a RETR FTP command to fetch the specified file from the remote FTP server
func (c *FtpServerConn) Retr(path string) error {
	code, msg, err := c.SendCmd(-1, "RETR %s", path)
	if err != nil {
		return err
	}
	if code != 125 && code != 150 {
		return &textproto.Error{Code: code, Msg: msg}
	}
	return err
}

// Stor issues a STOR FTP command to store a file to the remote FTP server.
func (c *FtpServerConn) Stor(path string) error {
	code, msg, err := c.SendCmd(-1, "STOR %s", path)
	if err != nil {
		return err
	}
	if code != 125 && code != 150 {
		return &textproto.Error{Code: code, Msg: msg}
	}
	return err
}

// RetrFile issues a RETR FTP command to fetch the specified file from the remote FTP server
func (c *FtpServerConn) RetrFile(remote, local string) error {
	reader, err := c.RetrRequest(remote)
	if err != nil {
		return err
	}
	defer reader.Close()

	file, err := os.Create(local)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, 32*1024)
	for {
		nr, err := reader.Read(buf)
		if nr > 0 {
			nw, err := file.Write(buf[:nr])
			if err != nil {
				return err
			}
			if nr != nw {
				return io.ErrShortWrite
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// StorFile issues a STOR FTP command to store a file to the remote FTP server.
func (c *FtpServerConn) StorFile(local, remote string) error {
	file, err := os.Open(local)
	if err != nil {
		return err
	}
	defer file.Close()

	writer, err := c.StorRequest(remote)
	if err != nil {
		return err
	}
	defer writer.Close()

	buf := make([]byte, 32*1024)
	for {
		nr, err := file.Read(buf)
		if nr > 0 {
			nw, err := writer.Write(buf[:nr])
			if err != nil {
				return err
			}
			if nr != nw {
				return io.ErrShortWrite
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// SendCmd Send a simple command string to the server and return the code and response string.
func (c *FtpServerConn) SendCmd(expectCode int, format string, args ...interface{}) (int, string, error) {

	if strings.HasPrefix(format, "PASS") {
		c.log("PASS ***")
	} else {
		c.logf(format, args...)
	}

	err := c.putCmd(format, args...)
	if err != nil {
		return 0, "", err
	}

	return c.getResponse(expectCode)
}

// Pasv issues a "PASV" command to get a port number for a data connection.
func (c *FtpServerConn) Pasv() (host string, port int, err error) {
	_, line, err := c.SendCmd(227, "PASV")
	if err != nil {
		return
	}
	// PASV response format : 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
	return parse227(line)
}

// Epsv issues a "EPSV" command to get a port number for a data connection.
func (c *FtpServerConn) Epsv() (port int, err error) {
	_, line, err := c.SendCmd(229, "EPSV")
	if err != nil {
		return
	}
	// EPSV response format : 229 Entering Extended Passive Mode (|||p|).
	return parse229(line)
}

// Port issues a PORT FTP command
func (c *FtpServerConn) Port(host string, port int) error {
	hostbytes := strings.Split(host, ".")
	portbytes := []string{strconv.Itoa(port / 256), strconv.Itoa(port % 256)}
	param := strings.Join(append(hostbytes, portbytes...), ",")
	_, _, err := c.SendCmd(CommandOkay, "PORT %s", param)
	return err
}

func (c *FtpServerConn) port(addr net.Addr) error {
	hostport := addr.String()
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return err
	}
	portVal, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	return c.Port(host, portVal)
}

// Eprt issues a EPRT FTP command
func (c *FtpServerConn) Eprt(host string, port int) error {
	addressfamily := 2
	ip := net.ParseIP(host)
	if ip.To4() != nil {
		addressfamily = 1
	}
	_, _, err := c.SendCmd(CommandOkay, "EPRT |%d|%s|%d|", addressfamily, host, port)
	return err
}

func (c *FtpServerConn) eprt(addr net.Addr) error {
	hostport := addr.String()
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return err
	}

	portVal, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	return c.Eprt(host, portVal)
}

// Auth issues a AUTH FTP command
func (c *FtpServerConn) Auth(param string) error {
	_, _, err := c.SendCmd(234, "AUTH %s", param)
	return err
}

// Pbsz issues a PBSZ FTP command
func (c *FtpServerConn) Pbsz(param string) error {
	_, _, err := c.SendCmd(CommandOkay, "PBSZ %s", param)
	return err
}

// Prot issues a PROT FTP command
func (c *FtpServerConn) Prot(param string) error {
	_, _, err := c.SendCmd(CommandOkay, "PROT %s", param)
	return err
}

// Feat issues a FEAT FTP command
func (c *FtpServerConn) Feat() error {
	_, _, err := c.SendCmd(211, "FEAT")
	return err
}

// Opts issues a OPTS FTP command
func (c *FtpServerConn) Opts(param string) error {
	_, _, err := c.SendCmd(200, "OPTS %s", param)
	return err
}

// GetResponse issues a FTP command response
func (c *FtpServerConn) GetResponse(expectCode int, timeout time.Duration) (int, string, error) {
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	return c.readResponse(expectCode)
}

// putCmd is a helper function to execute a command.
func (c *FtpServerConn) putCmd(format string, args ...interface{}) error {
	c.conn.SetWriteDeadline(time.Now().Add(c.readWriteTimeout))
	_, err := c.textprotoConn.Cmd(format, args...)
	return err
}

// getResponse is a helper function to check for the expected FTP return code
func (c *FtpServerConn) getResponse(expectCode int) (int, string, error) {
	c.conn.SetReadDeadline(time.Now().Add(c.readWriteTimeout))
	return c.readResponse(expectCode)
}

// readResponse is a helper function to check for the expected FTP return code
func (c *FtpServerConn) readResponse(expectCode int) (int, string, error) {
	code, message, err := c.textprotoConn.ReadResponse(expectCode)
	if err != nil {
		return code, message, err
	}
	c.logf("%d %s", code, message)
	return code, message, err
}

func (c *FtpServerConn) log(args ...interface{}) {
	if c.logger != nil {
		c.logger.Log(args...)
	}
}

func (c *FtpServerConn) logf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Logf(format, args...)
	}
}

// getLines
func (c *FtpServerConn) getLines(r io.Reader) (lines []string, err error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return lines, err
	}
	return
}

// transferCmd
func (c *FtpServerConn) transferCmd(format string, args ...interface{}) (conn net.Conn, err error) {
	var listener net.Listener
	if c.passive {
		host, port, err := c.makePasv()
		if err != nil {
			return nil, err
		}

		conn, err = net.DialTimeout(network, net.JoinHostPort(host, strconv.Itoa(port)), c.readWriteTimeout)
		if err != nil {
			return nil, err
		}

		if c.tlsConfig != nil {
			conn = tls.Client(conn, c.tlsConfig)
		}
	} else {
		listener, err = c.makePort()
		if err != nil {
			return nil, err
		}
		defer listener.Close()
	}

	code, msg, err := c.SendCmd(-1, format, args...)
	if err != nil {
		return nil, err
	}
	if code != 125 && code != 150 {
		return nil, &textproto.Error{Code: code, Msg: msg}
	}

	if listener != nil {
		conn, err = listener.Accept()
		if err != nil {
			return nil, err
		}

		if c.tlsConfig != nil {
			conn = tls.Server(conn, c.tlsConfig)
			//c.stateTLSConn(conn)
		}
	}

	return
}

func (c *FtpServerConn) stateTLSConn(conn net.Conn) {
	tlsconn, ok := conn.(*tls.Conn)
	if ok {
		//c.log("upgraded connection to TLS")
		err := tlsconn.Handshake()
		if err != nil {
			c.logf("handshake error: %v", err)
		}
		state := tlsconn.ConnectionState()
		c.logf("handshake complete: %v", state.HandshakeComplete)
	}
}

func (c *FtpServerConn) makePasv() (host string, port int, err error) {
	addr := c.conn.RemoteAddr()
	hostport := addr.String()
	host, _, err = net.SplitHostPort(hostport)
	if err != nil {
		return
	}

	ip := net.ParseIP(host)
	if ip.To4() != nil {
		return c.Pasv()
	}

	port, err = c.Epsv()
	return
}

func (c *FtpServerConn) makePort() (net.Listener, error) {
	addr := c.conn.LocalAddr()
	network := addr.Network()
	hostport := addr.String()
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, err
	}

	newaddr := net.JoinHostPort(host, "0")
	listenging := startListen(network, newaddr, c.readWriteTimeout)
	listener := <-listenging
	if listener == nil {
		return nil, errors.New("Unable to create listener")
	}

	listenerAddr := listener.Addr()
	ip := net.ParseIP(host)
	if ip.To4() != nil {
		if err = c.port(listenerAddr); err != nil {
			return nil, err
		}
		return listener, err
	}

	if err = c.eprt(listenerAddr); err != nil {
		return nil, err
	}
	return listener, err
}

// startListen
func startListen(network, laddr string, timeout time.Duration) chan net.Listener {
	listening := make(chan net.Listener)
	go func() {
		listener, err := net.Listen(network, laddr)
		if err != nil {
			listening <- nil
			return
		}

		if l, ok := listener.(*net.TCPListener); ok {
			l.SetDeadline(time.Now().Add(timeout))
		}

		listening <- listener
	}()
	return listening
}

// parse229
func parse229(msg string) (port int, err error) {
	matches := regexp229.FindStringSubmatch(msg)
	if matches == nil {
		err = errors.New("No matching pattern for message: " + msg)
		return
	}

	port, _ = strconv.Atoi(matches[1])
	return
}

// parse227
func parse227(msg string) (host string, port int, err error) {
	matches := regexp227.FindStringSubmatch(msg)
	if matches == nil {
		err = errors.New("No matching pattern for message: " + msg)
		return
	}

	// h1,h2,h3,h4,p1,p2
	// matches[0] = h1,h2,h3,h4,p1,p2
	// matches[1] = h1
	numbers := matches[1:]
	host = strings.Join(numbers[:4], ".")
	p1, _ := strconv.Atoi(numbers[4])
	p2, _ := strconv.Atoi(numbers[5])
	port = (p1 << 8) + p2
	return
}

// parse257
func parse257(msg string) (string, error) {
	start := strings.Index(msg, "\"")
	end := strings.LastIndex(msg, "\"")
	if start == -1 || end == -1 {
		return "", errors.New("Unsuported response format")
	}

	return msg[start+1 : end], nil
}

// Read implements the io.Reader interface on a FTP data connection.
func (d *FtpDataConn) Read(buf []byte) (int, error) {
	d.conn.SetReadDeadline(time.Now().Add(d.c.readWriteTimeout))
	return d.conn.Read(buf)
}

// Write implements the io.Writer interface on a FTP data connection.
func (d *FtpDataConn) Write(buf []byte) (int, error) {
	d.conn.SetWriteDeadline(time.Now().Add(d.c.readWriteTimeout))
	return d.conn.Write(buf)
}

// Close implements the io.Closer interface on a FTP data connection.
func (d *FtpDataConn) Close() error {
	err := d.conn.Close()
	_, _, err2 := d.c.getResponse(226)
	if err2 != nil {
		err = err2
	}
	return err
}
