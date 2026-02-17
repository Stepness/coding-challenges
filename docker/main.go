package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
	// TODO: remove child
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNET,
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

	err := cmd.Start()
	if err != nil {
		fmt.Printf("ERROR: The command failed to run: %v\n", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
	}

	userCgroupPath := getWritableCgroupPath()
	fmt.Println("User dedicated cgroup: " + userCgroupPath)

	err = os.MkdirAll(userCgroupPath, 0755)
	if err != nil {
		fmt.Printf("Error creating user cgroup path: %v\n", err)
	}
	defer func() {
		if err := os.Remove(userCgroupPath); err != nil {
			fmt.Printf("Error user Cgroup path removal: %v\n", err)
		}
	}()

	err = os.WriteFile(userCgroupPath+"/cgroup.procs", []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	if err != nil {
		fmt.Printf("Error assigning PID: %v\n", err)
	}

	err = os.WriteFile(userCgroupPath+"/memory.max", []byte("50000000"), 0644)
	if err != nil {
		fmt.Printf("Error set memory limit: %v\n", err)
	}

	cmd.Wait()
}

func child(args []string) {
	fmt.Println("Child args", args)

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

	err = syscall.Mount("proc", "/proc", "proc", 0, "")
	if err != nil {
		fmt.Printf("Error proc mount: %v\n", err)
		os.Exit(1)
	}

	err = syscall.Mount("sysfs", "/sys", "sysfs", 0, "")
	if err != nil {
		fmt.Printf("Error sysfs mount: %v\n", err)
		os.Exit(1)
	}

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

// To configure cgroup restrictions you can set them in the /sys/fs/cgroup folder,
// but to apply changes to this folder you need root access.
// To be able to run this app rootlessly, you can instead create cgroup files in a folder dedicated to the current user.
// The folder you can use can be calculated by reading /proc/self/cgroup file.
// For semplicity I copy-pasted the base format.
func getWritableCgroupPath() string {
	userUid := os.Geteuid()
	basePath := fmt.Sprintf("/sys/fs/cgroup/user.slice/user-%d.slice/user@%d.service", userUid, userUid)

	return basePath + "/mycontainer"
}
