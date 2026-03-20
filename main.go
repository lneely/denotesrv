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

var mountPath = flag.String("mount", "", "FUSE mount path (default: $HOME/mnt/denote)")

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
	args := []string{"fgstart"}
	if *mountPath != "" {
		args = append(args, "-mount", *mountPath)
	}
	cmd := exec.Command(exe, args...)
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

	// Setup FUSE mount
	mnt := *mountPath
	if mnt == "" {
		mnt = os.Getenv("DENOTE_9MOUNT")
	}
	if mnt == "" {
		mnt = filepath.Join(os.Getenv("HOME"), "mnt", "denote")
	}
	var fuseCmd *exec.Cmd
	if err := os.MkdirAll(mnt, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot create mount dir: %v\n", err)
	} else {
		fuseCmd = exec.Command("9pfuse", sockPath, mnt)
		if err := fuseCmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: 9pfuse failed: %v\n", err)
			fuseCmd = nil
		} else {
			fmt.Printf("mounted at %s\n", mnt)
		}
	}

	// Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("shutting down")
	if fuseCmd != nil {
		exec.Command("fusermount", "-u", mnt).Run()
		fuseCmd.Wait()
	}
	listener.Close()
	os.Remove(sockPath)
	os.Remove(pidPath)
}
