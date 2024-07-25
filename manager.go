package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	LOCAL_PORT  = 9191
	PUBLIC_PORT = 9292
	IMAGES_DIR  = "/home/plto/images_dir"
	SSH_PASS    = " "
	PEER_USER   = "root"
)

// dump saves the state of the process with the given pid to a directory
func dump(pid int) error {
	path := filepath.Join(IMAGES_DIR, fmt.Sprint(pid))
	if err := os.MkdirAll(path, 0755); err != nil {
		print(path)
		log.Println(err)
		return err
	}
	cmd := exec.Command("criu", "dump", "--shell-job", "-D", path, "-t", fmt.Sprint(pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Error dumping process with pid:", pid, ".", err, string(out))
		return err
	}
	return nil
}

// restore restores the state of the process from the directory with the given pid
func restore(pid int) error {
	path := filepath.Join(IMAGES_DIR, fmt.Sprint(pid))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Println("Can not find images for process with PID:", pid, err)
		return err
	}
	cmd := exec.Command("gnome-terminal", "--", "sudo", "criu", "restore", "--skip-file-rwx-check", "--shell-job", "-D", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Can not restore process with PID:", pid, err, string(out))
		return err
	}
	return nil

	// go func() {
	// 	cmd := "-c"
	// 	command := "sudo criu restore --link-remap --tcp-established --shell-job -D" + path
	// 	err := syscall.Exec("/bin/bash", []string{"/bin/bash", cmd, command}, os.Environ())
	// 	if err != nil {
	// 		log.Fatal("Error restoring process with PID:", pid, ".", err)
	// 	}
	// }()
}

func getFilesPath(pid int) ([]string, error) {

	cmd := exec.Command("lsof", "-p", fmt.Sprint(pid))

	// Run the command and get the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Could not execute command: %s\n", err)
		return nil, err
	}

	var out []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "REG") {
			fields := strings.Fields(line)
			if len(fields) > 8 {
				out = append(out, fields[8])
			}
		}
	}

	return out, nil
}

// handleSend handles the request from the local client and sends the image directory to the server
func handleSend(c net.Conn, peer_address string) {
	defer c.Close()
	buffer := make([]byte, 1024)
	n, err := c.Read(buffer)
	if err != nil {
		log.Fatal("Error getting command from process:", err)
		return
	}
	receivedString := string(buffer[:n])
	s := strings.Split(receivedString, ",")
	action := s[0]
	pid, _ := strconv.Atoi(s[1])
	log.Println("Recieved command", action, "from process with PID:", pid)
	if action == "STP" {

		log.Println("getting dependant files of process with PID:", pid)
		paths, err := getFilesPath(pid)
		if err != nil {
			log.Fatal("Error getting dependant files:", err)
		}

		log.Println("Dumping process with PID:", pid)
		err = dump(pid)
		if err != nil {
			log.Fatal("Error dumping process:", err)
		}

		// creating an array of source and destiantion paths
		soreseDestinationPath := make([][2]string, len(paths))
		for i, s := range paths {
			soreseDestinationPath[i] = [2]string{s, s}
		}
		soreseDestinationPath = append(soreseDestinationPath, [2]string{filepath.Join(IMAGES_DIR, fmt.Sprint(pid)), IMAGES_DIR})

		log.Println("Sending files of process with PID:", pid, "to the server")
		err = sendFiles(soreseDestinationPath, SSH_PASS, PEER_USER, peer_address)
		if err != nil {
			log.Fatal("Error sending files:", err)
		}

		err = sendSignal(pid, peer_address)
		if err != nil {
			log.Fatal("Error sending signal:", err)
		}
	}
}

func sendFiles(paths [][2]string, sshPass, remoteUser, serverIP string) error {

	for _, path := range paths {
		cmd := exec.Command("bash")
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		cmdWriter, _ := cmd.StdinPipe()
		cmd.Start()

		cmdString := "sshpass" + " -p " + "\"" + sshPass + "\"" + " rsync" + " -e \"ssh -o StrictHostKeyChecking=no\"" + " --mkpath" + " -rzP " + path[0] + fmt.Sprintf(" %s@%s:%s", remoteUser, serverIP, path[1])

		_, err := cmdWriter.Write([]byte(cmdString + "\n"))
		cmdWriter.Write([]byte("exit" + "\n"))

		cmd.Wait()
		if err != nil {
			log.Printf("Error executing rsync, when sending file: %s error: %s\n", path, fmt.Sprint(err)+": "+stderr.String())
			return err
		}

	}
	return nil
}

// sendImageDir compresses and sends the image directory with the given pid to the server
func sendSignal(pid int, peer_address string) error {
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", peer_address, PUBLIC_PORT))
	if err != nil {
		log.Println("Can not connect to server:", err)
		return err
	}
	defer c.Close()

	// Send the file name first
	_, err = fmt.Fprintf(c, "%s\n", fmt.Sprint(pid))
	if err != nil {
		log.Println("Error writing PID to socket")
		return err
	}
	return nil
}

// handleRecv handles the request from the server and restores the image directory
func handleRecv(c net.Conn) {
	defer c.Close()
	reader := bufio.NewReader(c)
	pidS, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("Error reading signal", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(pidS)) // Remove the newline character
	log.Println("Received images with PID:", pid)
	log.Println("Restoring process with PID:", pid)
	err = restore(pid)
	if err != nil {
		log.Fatal("Error restoring process with PID:", pid, err)
	}
}

// startServer starts the server that listens for requests from the clients
func startServer() {
	s, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", PUBLIC_PORT))
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
	defer s.Close()
	for {
		c, err := s.Accept()
		if err != nil {
			log.Println("Error accepting new connection (server):", err)
			continue
		}
		go handleRecv(c)
	}
}

// startDaemon starts the daemon that listens for requests from the local processes
func startDaemon(peer_address string) {
	s, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", LOCAL_PORT))
	if err != nil {
		log.Fatal("Error starting deamon:", err)
	}
	defer s.Close()
	for {
		c, err := s.Accept()
		if err != nil {
			log.Println("Error accepting new connection (deamon):", err)
			continue
		}
		go handleSend(c, peer_address)
	}
}

func main() {
	peer_address := os.Getenv("PA")
	log.Println(peer_address)
	log.Println("Manager started")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		log.Println("Deamon started")
		startDaemon(peer_address)
		wg.Done()
	}()
	go func() {
		log.Println("Server started")
		startServer()
		wg.Done()
	}()
	wg.Wait()
}
