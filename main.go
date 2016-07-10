/**
 * @file main.go
 * @author Mikhail Klementyev jollheef<AT>riseup.net
 * @license GNU AGPLv3
 * @date July, 2016
 */

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	system "github.com/jollheef/go-system"
	"github.com/naoina/toml"
)

type config struct {
	Network struct {
		Addr string
	}
	Qemu struct {
		Config string
		Port   int
	}
}

func readConfig(path string) (cfg config, err error) {

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	err = toml.Unmarshal(buf, &cfg)
	if err != nil {
		return
	}

	return
}

func getRandomAddr() (addr string) {
	// 127.1-255.0-255.0-255:10000-50000
	ip := fmt.Sprintf("127.%d.%d.%d",
		rand.Int()%254+1, rand.Int()%255, rand.Int()%254)
	port := rand.Int()%40000 + 10000
	return fmt.Sprintf("%s:%d", ip, port)
}

func getFreeAddr() (addr string) {

	for {
		addr = getRandomAddr()

		stdout, _, _, err := system.System("netstat", "-tupln")
		if err != nil {
			panic(err)
		}

		if !strings.Contains(stdout, addr) {
			break
		}
	}

	return
}

func startVM(localPort int, config, addr string) {
	hostfwd := fmt.Sprintf("hostfwd=tcp:%s-:%d", addr, localPort)
	system.System("qemu-system-x86_64", "-readconfig", config,
		"-snapshot", "-nographic",
		"-netdev", "user,id=net0,"+hostfwd)
	fmt.Println(addr, "closed")
}

func killVM(addr string) {
	system.System("bash", "-c",
		"kill $(ps aux | grep "+addr+" | grep -v grep | awk '{print $2}')")
}

func forward(localConn net.Conn, cfg config) {

	addr := getFreeAddr()

	go startVM(cfg.Qemu.Port, cfg.Qemu.Config, addr)

	time.Sleep(time.Second)

	remoteConn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	copyConn := func(writer, reader net.Conn) {
		defer killVM(addr)
		defer localConn.Close()
		_, err := io.Copy(writer, reader)
		if err != nil {
			fmt.Println("io.Copy error: %s", err)
		}
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}

func main() {

	cfg, err := readConfig("qtun.conf.example")
	if err != nil {
		panic(err)
	}

	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go forward(conn, cfg)
	}
}
