// denotesrv - 9P server for denote notes
package main

import (
	fs "denotesrv/internal/p9/server"
	"denotesrv/pkg/config"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"9fans.net/go/plan9/client"
)

const serviceName = "denote"

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: denotesrv <start|stop|fgstart|status>")
		os.Exit(1)
	}

	ns := client.Namespace()
	if ns == "" {
		fmt.Fprintln(os.Stderr, "no namespace")
		os.Exit(1)
	}

	sockPath := filepath.Join(ns, serviceName)
	pidPath := filepath.Join(ns, serviceName+".pid")

	switch flag.Arg(0) {
	case "start":
		if isRunning(sockPath) {
			fmt.Println("denotesrv already running")
			os.Exit(0)
		}
		daemonize(pidPath)
	case "fgstart":
		if isRunning(sockPath) {
			fmt.Println("denotesrv already running")
			os.Exit(0)
		}
		runServer(sockPath, pidPath)
	case "stop":
		stopServer(sockPath, pidPath)
	case "status":
		if isRunning(sockPath) {
			fmt.Println("denotesrv running")
		} else {
			fmt.Println("denotesrv not running")
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: denotesrv <start|stop|fgstart|status>")
		os.Exit(1)
	}
}

func isRunning(sockPath string) bool {
	conn, err := net.Dial("unix", sockPath)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func daemonize(pidPath string) {
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "fgstart")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("denotesrv started (pid %d)\n", cmd.Process.Pid)
}

func stopServer(sockPath, pidPath string) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("denotesrv not running")
		return
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	if pid > 0 {
		syscall.Kill(pid, syscall.SIGTERM)
	}
	os.Remove(pidPath)
	os.Remove(sockPath)
	fmt.Println("denotesrv stopped")
}

func runServer(sockPath, pidPath string) {
	// Remove stale socket
	if _, err := os.Stat(sockPath); err == nil {
		os.Remove(sockPath)
	}

	// Write PID file
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	// Get denote directory
	denoteDir := config.DefaultDenoteDir
	if envDir := os.Getenv("DENOTE_DIR"); envDir != "" {
		denoteDir = envDir
	}

	// Start server
	srv, err := fs.NewServer(denoteDir, fs.Callbacks{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create server: %v\n", err)
		os.Exit(1)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go srv.Serve(conn)
		}
	}()

	fmt.Printf("denotesrv listening on %s (dir: %s)\n", sockPath, denoteDir)

	// Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("shutting down")
	listener.Close()
	os.Remove(sockPath)
	os.Remove(pidPath)
}
