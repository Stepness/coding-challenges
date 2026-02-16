package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	if os.Getenv("IS_CHILD_PROCESS") == "true" {
		child(os.Args)
		return
	}

	switch os.Args[1] {
	case "run":
		run(os.Args)
	default:
		fmt.Printf("Unknown command: %v\n", os.Args[1])
	}
}

func run(args []string) {
	fmt.Println("Parent args", args)
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "IS_CHILD_PROCESS=true")

	var err = cmd.Run()
	var exitErr *exec.ExitError
	if err != nil {
		fmt.Printf("ERROR: The command failed to run: %v\n", err)
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
	}
}

func child(args []string) {
	fmt.Println("Child args", args)

	syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")

	err := syscall.Chroot("./resources")
	if err != nil {
		fmt.Printf("Error setting chroot: %v\n", err)
		os.Exit(1)
	}

	err = os.Chdir("/")
	if err != nil {
		fmt.Printf("Chdir error: %v\n", err)
		os.Exit(1)
	}

	syscall.Mount("proc", "/proc", "proc", 0, "")
	err = syscall.Sethostname([]byte("container"))
	if err != nil {
		fmt.Printf("Error setting hostname: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(args[2], args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Child error: %v\n", err)
		os.Exit(1)
	}

}
