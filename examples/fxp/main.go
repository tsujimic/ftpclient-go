package main

import (
    "fmt"
    "flag"
    "net"
    "strconv"
    "sync"
    "time"
    "log"
    "github.com/tsujimic/ftpclient-go"
)

func main() {

    // server-to-server FTP transfer 
    // source
    var host1, user1, pass1, path1 string
    var port1 int
    flag.StringVar(&host1, "host1", "", "source host name.")
    flag.IntVar(&port1, "port1", 21, "source tcp/ip port number.")
    flag.StringVar(&user1, "user1", "", "source login user.")
    flag.StringVar(&pass1, "pass1", "", "source login password.")
    flag.StringVar(&path1, "path1", "", "source file path.")

    // destination    
    var host2, user2, pass2, path2 string
    var port2 int
    flag.StringVar(&host2, "host2", "", "destination host name.")
    flag.IntVar(&port2, "port2", 21, "destination tcp/ip port number.")
    flag.StringVar(&user2, "user2", "", "destination login user.")
    flag.StringVar(&pass2, "pass2", "", "destination login password.")
    flag.StringVar(&path2, "path2", "", "destination file path.")

    // timeout server-to-server FTP transfer 
    var timeout int
    flag.IntVar(&timeout, "timeout", 600, "timeout seconds server-to-server FTP transfer (default 600).")
    flag.Parse()

    log.Println(fmt.Sprintf("source: %s %s", host1, path1))
    log.Println(fmt.Sprintf("destination: %s %s", host2, path2))
    log.Println(fmt.Sprintf("timeout: %d seconds", timeout))

    // connect source    
    sourceAddr := net.JoinHostPort(host1, strconv.Itoa(port1))
    source, err := ftpclient.Dial(sourceAddr)
    if err != nil {
        panic(err)
    }
    defer source.Quit()

    err = source.Login(user1, pass1)
    if err != nil {
        panic(err)
    }
    
    err = source.Type("I")
    if err != nil {
        panic(err)
    }

    // connect destination
    destinationAddr := net.JoinHostPort(host2, strconv.Itoa(port2))
    destination, err := ftpclient.Dial(destinationAddr)
    if err != nil {
        panic(err)
    }
    defer destination.Quit()

    err = destination.Login(user2, pass2)
    if err != nil {
        panic(err)
    }

    err = destination.Type("I")
    if err != nil {
        panic(err)
    }

    // start server-to-server FTP transfer    
    log.Println("Start server-to-server FTP transfer")
    
    host, port, err := source.Pasv()
    if err != nil {
        panic(err)
    }
    
    err = destination.Port(host, port)
    if err != nil {
        panic(err)
    }

    err = destination.Stor(path2)
    if err != nil {
        panic(err)
    }
    
    err = source.Retr(path1)
    if err != nil {
        panic(err)
    }

    // wait response    
    var wg sync.WaitGroup
    
    //tm := time.Now().Add(time.Duration(timeout) * time.Minute)
    tm := time.Now().Add(time.Duration(timeout) * time.Second)
    wg.Add(1)
    go func() {
        source.ReadResponse(226, tm)
        defer wg.Done()
    }()

    wg.Add(1)
    go func() {
        destination.ReadResponse(226, tm)
        defer wg.Done()
    }()
    
    wg.Wait()
    log.Println("Done!!!")
}

