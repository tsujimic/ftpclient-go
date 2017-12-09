package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/tsujimic/ftpclient-go"
)

func main() {
	var host, user, pass, remote, tracefile string
	var port int
	flag.StringVar(&host, "host", "", "target host name")
	flag.IntVar(&port, "port", 21, "tcp/ip port number")
	flag.StringVar(&user, "user", "", "login username")
	flag.StringVar(&pass, "pass", "", "login password")
	flag.StringVar(&remote, "remote", "", "remote file path")
	flag.StringVar(&tracefile, "trace", "", "trace file path")
	flag.Parse()

	if tracefile != "" {
		file, err := os.OpenFile(tracefile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		log.SetOutput(io.MultiWriter(file, os.Stderr))
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	log.Println("Start")
	start := time.Now()

	cfg := ftpclient.NewConfig()
	client := ftpclient.New(cfg)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err := client.DialTimeout(addr, 30*time.Second)
	if err != nil {
		panic(err)
	}
	defer client.Quit()

	err = client.Login(user, pass)
	if err != nil {
		panic(err)
	}

	client.SetPasv(true)
	infos, err := client.Dir(remote)
	if err != nil {
		panic(err)
	}

	for _, v := range infos {
		log.Println(fmt.Sprintf(" Name: %s Size: %d", v.Name(), v.Size()))
	}

	sec := (time.Now().Sub(start)).Seconds()
	log.Printf("Stopwatch : %f seconds", sec)
}
