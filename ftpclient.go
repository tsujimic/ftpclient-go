// Package ftpclient implements a FTP client as described in RFC 959.
package ftpclient

import (
	"bufio"
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

// FtpServerConn represents the connection to a remote FTP server.
type FtpServerConn struct {
    passive         bool
	textprotoConn   *textproto.Conn
    conn            net.Conn
	timeout         time.Duration
}

// FtpDataConnector represent a data-connection
type FtpDataConnector struct {
	conn net.Conn
	c    *FtpServerConn
}

var regexp227 *regexp.Regexp

func init() {
	regexp227, _ = regexp.Compile("([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+)")
}

// Connect to the given host and port. 
func Connect(addr string) (*FtpServerConn, error) {
	return Dial(addr)
}

// Dial to the given host and port. 
func Dial(addr string) (*FtpServerConn, error) {
	return DialTimeout(addr, 0)
}

// DialTimeout to the given host and port. 
func DialTimeout(addr string, timeout time.Duration) (*FtpServerConn, error) {
	conn, err := net.DialTimeout("tcp4", addr, timeout)
	if err != nil {
		return nil, err
	}
    
	textprotoConn := textproto.NewConn(conn)
	c := &FtpServerConn{
        passive:        false,
		textprotoConn:  textprotoConn,
        conn:           conn,
		timeout:        timeout,
	}

	_, _, err = c.getResponse(220)
	if err != nil {
		c.Quit()
		return nil, err
	}
    
	return c, nil
}

// Login as the given user. 
func (c *FtpServerConn) Login(user, password string) error {
	code, message, err := c.SendCmd(-1, "USER %s", user)
	if err != nil {
		return err
	}

    if code == 331 {
		_, _, err = c.SendCmd(230, "PASS %s", password)
		if err != nil {
			return err
		}
    	return nil
    }
    
    return errors.New(message)
}

// Type issues a TYPE FTP command
func (c *FtpServerConn) Type(param string) error {
	_, _, err := c.SendCmd(200, "TYPE %s", param)
    return err
}

// Cwd issues a CWD FTP command, which changes the current directory to the specified path.
func (c *FtpServerConn) Cwd(path string) error {
	_, _, err := c.SendCmd(250, "CWD %s", path)
	return err
}

// Cdup issues a CDUP FTP command, which changes the current directory to the parent directory.
// This is similar to a call to ChangeDir with a path set to "..".
func (c *FtpServerConn) Cdup() error {
	_, _, err := c.SendCmd(250, "CDUP")
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
	_, _, err = c.SendCmd(250, "RNTO %s", to)
	return err
}

// Delete issues a DELE FTP command to delete the specified file from the remote FTP server.
func (c *FtpServerConn) Delete(path string) error {
	code, msg, err := c.SendCmd(-1, "DELE %s", path)
    if err != nil {
        return err
    }
    if code != 250 && code != 200 {
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
	_, _, err := c.SendCmd(250, "RMD %s", path)
	return err
}

// Noop has no effects and is usually used to prevent the remote FTP server to close the otherwise idle connection.
func (c *FtpServerConn) Noop() error {
	_, _, err := c.SendCmd(200, "NOOP")
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

// Quit issues a QUIT FTP command to properly close the connection from the remote FTP server.
func (c *FtpServerConn) Quit() error {
    //log.Println("called quit")
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
func (c *FtpServerConn) NlstRequest(args ... string) (io.ReadCloser, error) {
    cmd := append([]string {"NLST"}, args...)
    val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return nil, err
	}

	return &FtpDataConnector{conn, c}, nil
}

// ListRequest issues a LIST FTP command.
func (c *FtpServerConn) ListRequest(args ... string) (io.ReadCloser, error) {
    cmd := append([]string {"LIST"}, args...)
    val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return nil, err
	}

	return &FtpDataConnector{conn, c}, nil
}

// RetrRequest issues a RETR FTP command to fetch the specified file from the remote FTP server
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *FtpServerConn) RetrRequest(path string) (io.ReadCloser, error) {
	conn, err := c.transferCmd("RETR %s", path)
	if err != nil {
		return nil, err
	}
	return &FtpDataConnector{conn, c}, nil
}

// StorRequest issues a STOR FTP command to store a file to the remote FTP server.
// The returned WriteCloser must be closed to cleanup the FTP data connection.
func (c *FtpServerConn) StorRequest(path string) (io.WriteCloser, error) {
	conn, err := c.transferCmd("STOR %s", path)
	if err != nil {
		return nil, err
	}
	return &FtpDataConnector{conn, c}, nil
}

// SetPasv sets the mode to passive or active for data transfers.
func (c *FtpServerConn) SetPasv(ispassive bool) {
	c.passive = ispassive
}

// Nlst issues an NLST FTP command.
func (c *FtpServerConn) Nlst(args ... string) (lines []string, err error) {
    cmd := append([]string {"NLST"}, args...)
    val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return
	}

	r := &FtpDataConnector{conn, c}
	defer r.Close()
    
    lines, err = c.getLines(r)
	if err != nil {
		return
	}
	return
}

// List issues a LIST FTP command.
func (c *FtpServerConn) List(args ... string) (lines []string, err error) {
    cmd := append([]string {"LIST"}, args...)
    val := strings.Join(cmd, " ")
	conn, err := c.transferCmd(val)
	if err != nil {
		return
	}

	r := &FtpDataConnector{conn, c}
	defer r.Close()
    
    lines, err = c.getLines(r)
	if err != nil {
		return
	}
	return
}

// Dir issues a LIST FTP command.
func (c *FtpServerConn) Dir(args ... string) (infos []os.FileInfo, err error) {
    cmd := append([]string {"LIST"}, args...)
    val := strings.Join(cmd, " ")
    //log.Println(val)
	conn, err := c.transferCmd(val)
	if err != nil {
		return
	}

	r := &FtpDataConnector{conn, c}
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
func (c *FtpServerConn) Retr(path string) (error) {
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


// Pasv issues a "PASV" command to get a port number for a data connection.
func (c *FtpServerConn) Pasv() (host string, port int, err error) {
    //log.Println("called Pasv")
	_, line, err := c.SendCmd(227, "PASV")
	if err != nil {
		return
	}
    // PASV response format : 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
    return parse227(line)
}

// Port issues a PORT FTP command
func (c *FtpServerConn) Port(host string, port int) error {
    //log.Println("called Port")
    hostbytes := strings.Split(host, ".")
    portbytes := []string { strconv.Itoa(port / 256), strconv.Itoa(port % 256) }
    param := strings.Join(append(hostbytes, portbytes...), ",")    
	_, _, err := c.SendCmd(200, "PORT %s", param)
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

// ReadResponse issues a FTP command response
func (c *FtpServerConn)ReadResponse(expectCode int, t time.Time) (int, string, error) {
    c.conn.SetReadDeadline(t)
    return c.getResponse(expectCode)
}

// SendCmd Send a simple command string to the server and return the code and response string.
func (c *FtpServerConn) SendCmd(expectCode int, format string, args ...interface{}) (int, string, error) {
    err := c.putCmd(format, args...)
	if err != nil {
		return 0, "", err
	}

	return c.getResponse(expectCode)
}

// putCmd is a helper function to execute a command.
func (c *FtpServerConn) putCmd(format string, args ...interface{}) error {
	_, err := c.textprotoConn.Cmd(format, args...)
    return err
}

// getResponse is a helper function to check for the expected FTP return code
func (c *FtpServerConn) getResponse(expectCode int) (int, string, error) {
	return c.textprotoConn.ReadResponse(expectCode)
}

func (c *FtpServerConn) getLine() (string, error) {
	return c.textprotoConn.ReadLine()
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
            //log.Println(err)
            return nil, err
        }
        
       	conn, err = net.DialTimeout("tcp4", net.JoinHostPort(host, strconv.Itoa(port)), c.timeout)
        if err != nil {
            return nil, err
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
    }
    
    return    
}

// makePasv
func (c *FtpServerConn) makePasv() (host string, port int, err error) {
    //log.Println("called Pasv")
	_, line, err := c.SendCmd(227, "PASV")
	if err != nil {
		return
	}
    // PASV response format : 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
    return parse227(line)
}

// makePort
func (c *FtpServerConn) makePort() (net.Listener, error) {
    tcpaddr := c.conn.LocalAddr()
    network := tcpaddr.Network()
    
    localaddr, err := net.ResolveTCPAddr(network, tcpaddr.String())
    if err != nil {
        return nil, err
    }
    
    newaddr := localaddr.IP.String() + ":0"
    listenging := startListen(network, newaddr)
    list := <- listenging
    if list == nil {
        return nil, errors.New("Unable to create listener")
    }
    
    localaddr, err = net.ResolveTCPAddr(list.Addr().Network(), list.Addr().String())
    err = c.Port(localaddr.IP.String(), localaddr.Port)
    return list, err
}

// startListen
func startListen(network, laddr string) chan net.Listener {
    listening := make(chan net.Listener)
    go func() {
        ret, err := net.Listen(network, laddr)
        if err != nil {
            listening <- nil
            return
        }
        listening <- ret
    }()
    return listening
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
    // :
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
func (r *FtpDataConnector) Read(buf []byte) (int, error) {
	return r.conn.Read(buf)
}

// Write implements the io.Writer interface on a FTP data connection.
func (r *FtpDataConnector) Write(buf []byte) (int, error) {
	return r.conn.Write(buf)
}

// Close implements the io.Closer interface on a FTP data connection.
func (r *FtpDataConnector) Close() error {
	err := r.conn.Close()
    _, _, err2 := r.c.getResponse(226)
	if err2 != nil {
		err = err2
	}
	return err
}
