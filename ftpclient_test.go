package ftpclient

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestDelete(t *testing.T) {
	// go test -v -run TestDelete
	cases := []struct {
		Host       string
		Port       int
		User, Pass string
		Remote     string
		TLSEnable  bool
		Cert, Key  string
		Implicit   bool
	}{
		{"localhost", 21, "anonymous", "anonymous@example.com", "BigBuckBunny.mov", false, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", "BigBuckBunny.mov", true, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", "BigBuckBunny.mov", true, "cert.pem", "key.pem", false},
		//{"localhost", 990, "anonymous", "anonymous@example.com", "BigBuckBunny.mov", true, "cert.pem", "key.pem", true},
	}

	for _, c := range cases {
		err := delete(t, c.Host, c.Port, c.User, c.Pass, c.Remote, c.TLSEnable, c.Cert, c.Key, c.Implicit)
		if err != nil {
			t.Error(err)
		}
	}
}

func delete(t *testing.T, host string, port int, user, pass, remote string, tlsenable bool, cert, key string, implicit bool) error {
	var err error
	var tlscfg *tls.Config

	if tlsenable {
		if cert != "" && key != "" {
			tlscfg, err = NewTLSConfigWithX509KeyPair(cert, key)
			if err != nil {
				return err
			}

			tlscfg.InsecureSkipVerify = true
		} else {
			tlscfg = NewTLSConfig()
			tlscfg.InsecureSkipVerify = true
		}
	}

	logger := NewDefaultLogger()
	cfg := NewConfig().WithLogger(logger).WithTLSConfig(tlscfg).WithTLSImplicit(implicit)
	client := New(cfg)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err = client.DialTimeout(addr, 30*time.Second)
	if err != nil {
		return err
	}
	defer client.Quit()

	err = client.Login(user, pass)
	if err != nil {
		return err
	}

	err = client.Opts("UTF8 ON")
	if err != nil {
		return err
	}

	err = client.Delete(remote)
	if err != nil {
		return err
	}

	return nil
}

func TestPut(t *testing.T) {
	// go test -v -run TestPut
	cases := []struct {
		Host          string
		Port          int
		User, Pass    string
		Passive       bool
		Local, Remote string
		TLSEnable     bool
		Cert, Key     string
		Implicit      bool
	}{
		{"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "日本語BigBuckBunny.mov", false, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-480p-h264.mov", "日本語BigBuckBunny.mov", true, "", "", false},
		// {"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-480p-h264.mov", "BigBuckBunny.mov", false, "", "", false},
		// {"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "BigBuckBunny.mov", true, "cert.pem", "key.pem", false},
		// {"localhost", 990, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "BigBuckBunny.mov", true, "cert.pem", "key.pem", true},
	}

	for _, c := range cases {
		err := put(t, c.Host, c.Port, c.User, c.Pass, c.Passive, c.Local, c.Remote, c.TLSEnable, c.Cert, c.Key, c.Implicit)
		if err != nil {
			t.Error(err)
		}
	}
}

func put(t *testing.T, host string, port int, user, pass string, passive bool, local, remote string, tlsenable bool, cert, key string, implicit bool) error {
	var err error
	var tlscfg *tls.Config

	if tlsenable {
		if cert != "" && key != "" {
			tlscfg, err = NewTLSConfigWithX509KeyPair(cert, key)
			if err != nil {
				return err
			}

			tlscfg.InsecureSkipVerify = true
		} else {
			tlscfg = NewTLSConfig()
			tlscfg.InsecureSkipVerify = true
		}
	}

	logger := NewDefaultLogger()
	cfg := NewConfig().WithLogger(logger).WithTLSConfig(tlscfg).WithTLSImplicit(implicit)
	client := New(cfg)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err = client.DialTimeout(addr, 30*time.Second)
	if err != nil {
		return err
	}
	defer client.Quit()

	err = client.Login(user, pass)
	if err != nil {
		return err
	}

	err = client.Opts("UTF8 ON")
	if err != nil {
		return err
	}

	err = client.Type("I")
	if err != nil {
		return err
	}

	file, err := os.Open(local)
	if err != nil {
		return err
	}
	defer file.Close()

	fileinfo, err := file.Stat()
	if err != nil {
		return err
	}
	size := fileinfo.Size()

	client.SetPasv(passive)
	writer, err := client.StorRequest(remote)
	if err != nil {
		return err
	}
	defer writer.Close()

	start := time.Now()
	buf := make([]byte, 4*1024)
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

	now := time.Now()
	sec := (now.Sub(start)).Seconds()
	transferbps := (float64(size) / sec) * 8
	t.Logf("Stopwatch : %f seconds, %f Mbit/s", sec, transferbps/1048576)
	return nil
}

func TestGet(t *testing.T) {
	// go test -v -run TestGet
	cases := []struct {
		Host          string
		Port          int
		User, Pass    string
		Passive       bool
		Remote, Local string
		TLSEnable     bool
		Cert, Key     string
		Implicit      bool
	}{
		{"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "BigBuckBunny-720p-h264.mov", false, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "BigBuckBunny-720p-h264.mov", true, "", "", false},
		// {"localhost", 21, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "BigBuckBunny-720p-h264.mov", true, "cert.pem", "key.pem", false},
		// {"localhost", 990, "anonymous", "anonymous@example.com", true, "BigBuckBunny-720p-h264.mov", "BigBuckBunny-720p-h264.mov", true, "cert.pem", "key.pem", true},
	}

	for _, c := range cases {
		err := get(t, c.Host, c.Port, c.User, c.Pass, c.Passive, c.Remote, c.Local, c.TLSEnable, c.Cert, c.Key, c.Implicit)
		if err != nil {
			t.Error(err)
		}
	}
}

func get(t *testing.T, host string, port int, user, pass string, passive bool, remote, local string, tlsenable bool, cert, key string, implicit bool) error {
	var err error
	var tlscfg *tls.Config

	if tlsenable {
		if cert != "" && key != "" {
			tlscfg, err = NewTLSConfigWithX509KeyPair(cert, key)
			if err != nil {
				return err
			}

			tlscfg.InsecureSkipVerify = true
		} else {
			tlscfg = NewTLSConfig()
			tlscfg.InsecureSkipVerify = true
		}
	}

	logger := NewDefaultLogger()
	cfg := NewConfig().WithLogger(logger).WithTLSConfig(tlscfg).WithTLSImplicit(implicit)
	client := New(cfg)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err = client.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Quit()

	err = client.Login(user, pass)
	if err != nil {
		return err
	}

	err = client.Opts("UTF8 ON")
	if err != nil {
		return err
	}

	err = client.Type("I")
	if err != nil {
		return err
	}

	client.SetPasv(passive)
	// infos, err := client.Dir(remote)
	// if err != nil {
	// 	return err
	// }

	// length := len(infos)
	// if length == 0 {
	// 	return fmt.Errorf("file not found: %s", remote)
	// }

	// filesize := infos[0].Size()
	reader, err := client.RetrRequest(remote)
	if err != nil {
		return err
	}
	defer reader.Close()

	file, err := os.Create(local)
	if err != nil {
		return err
	}
	defer file.Close()
	// defer func() {
	// 	if err := file.Close(); err != nil {
	// 		return
	// 	}
	// }()

	var size int64
	start := time.Now()
	buf := make([]byte, 4*1024)
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
			size += int64(nw)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	now := time.Now()
	sec := (now.Sub(start)).Seconds()
	transferbps := (float64(size) / sec) * 8
	//fmt.Printf("Stopwatch : %f seconds, %f Mbit/s", sec, transferbps/1048576)
	t.Logf("Stopwatch : %f seconds, %f Mbit/s", sec, transferbps/1048576)
	return nil
}

func TestLogin(t *testing.T) {
	// go test -v -run TestLogin
	cases := []struct {
		Host       string
		Port       int
		User, Pass string
		Passive    bool
		TLSEnable  bool
		Cert, Key  string
		Implicit   bool
	}{
		{"localhost", 21, "anonymous", "anonymous@example.com", true, false, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", true, true, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", true, true, "cert.pem", "key.pem", false},
		//{"localhost", 990, "anonymous", "anonymous@example.com", true, true, "cert.pem", "key.pem", true},
	}

	for _, c := range cases {
		err := login(c.Host, c.Port, c.User, c.Pass, c.Passive, c.TLSEnable, c.Cert, c.Key, c.Implicit)
		if err != nil {
			t.Error(err)
		}
	}
}

func login(host string, port int, user, pass string, passive, tlsenable bool, cert, key string, implicit bool) error {
	var err error
	var tlscfg *tls.Config

	if tlsenable {
		if cert != "" && key != "" {
			tlscfg, err = NewTLSConfigWithX509KeyPair(cert, key)
			if err != nil {
				return err
			}

			tlscfg.InsecureSkipVerify = true
		} else {
			tlscfg = NewTLSConfig()
			tlscfg.InsecureSkipVerify = true
		}
	}

	logger := NewDefaultLogger()
	cfg := NewConfig().WithLogger(logger).WithTLSConfig(tlscfg).WithTLSImplicit(implicit)
	client := New(cfg)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err = client.DialTimeout(addr, 30*time.Second)
	if err != nil {
		return err
	}
	defer client.Quit()

	err = client.Login(user, pass)
	if err != nil {
		return err
	}
	return nil
}

func TestDir(t *testing.T) {
	// go test -v -run TestDir
	cases := []struct {
		Host       string
		Port       int
		User, Pass string
		Passive    bool
		TLSEnable  bool
		Cert, Key  string
		Implicit   bool
	}{
		{"localhost", 21, "anonymous", "anonymous@example.com", true, false, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", false, false, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", true, true, "", "", false},
		//{"localhost", 21, "anonymous", "anonymous@example.com", true, true, "cert.pem", "key.pem", false},
		//{"localhost", 990, "anonymous", "anonymous@example.com", true, true, "cert.pem", "key.pem", true},
		//{"localhost", 990, "anonymous", "anonymous@example.com", true, true, "", "", true},
	}

	for _, c := range cases {
		err := dir(c.Host, c.Port, c.User, c.Pass, c.Passive, c.TLSEnable, c.Cert, c.Key, c.Implicit)
		if err != nil {
			t.Error(err)
		}
	}
}

func dir(host string, port int, user, pass string, passive, tlsenable bool, cert, key string, implicit bool) error {
	var err error
	var tlscfg *tls.Config

	if tlsenable {
		if cert != "" && key != "" {
			tlscfg, err = NewTLSConfigWithX509KeyPair(cert, key)
			if err != nil {
				return err
			}

			tlscfg.InsecureSkipVerify = true
		} else {
			tlscfg = NewTLSConfig()
			tlscfg.InsecureSkipVerify = true
		}
	}

	logger := NewDefaultLogger()
	cfg := NewConfig().WithLogger(logger).WithTLSConfig(tlscfg).WithTLSImplicit(implicit)
	client := New(cfg)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err = client.DialTimeout(addr, 30*time.Second)
	if err != nil {
		return err
	}
	defer client.Quit()

	err = client.Login(user, pass)
	if err != nil {
		return err
	}

	err = client.Opts("UTF8 ON")
	if err != nil {
		return err
	}

	// err = client.Feat()
	// if err != nil {
	// 	return err
	// }

	err = client.Type("I")
	if err != nil {
		return err
	}

	client.SetPasv(passive)
	files, err := client.Dir("")
	if err != nil {
		return err
	}

	for _, file := range files {
		fmt.Printf("%s\n", file.Name())
	}

	return nil
}
