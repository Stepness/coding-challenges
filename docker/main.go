package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type RegistryAuthResponse struct {
	Token string
}

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Manifests     []struct {
		Digest    string `json:"digest"`
		MediaType string `json:"mediaType"`
		Platform  struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

var httpClient = &http.Client{}

func main() {
	if os.Getenv("IS_CHILD_PROCESS") == "true" {
		child()
		return
	}

	switch os.Args[1] {
	case "run":
		run()
	case "pull":
		if len(os.Args) < 3 {
			log.Fatalf("Received only %d param. Expected 3", len(os.Args))
		}
		pull()
	default:
		fmt.Printf("Unknown command: %v\n", os.Args[1])
	}
}

// The run method clones himself into a new child process by reexecuting the application again via
// /proc/self/exe, and setting the [IS_CHILD_PROCESS] env variable.
//
// The child process is then put in new namespaces and has cgroup configured.
func run() {
	cmd := exec.Command("/proc/self/exe", os.Args[2:]...)
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
	cmd.Env = append(cmd.Env, `PS1=\u@\h:\w\$ `)

	err := cmd.Start()
	if err != nil {
		fmt.Printf("ERROR: The command failed to run: %v\n", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
	}

	userCgroupPath := configureCgroup(cmd.Process.Pid)
	defer func() {
		if err := os.Remove(userCgroupPath); err != nil {
			fmt.Printf("Error user Cgroup path removal: %v\n", err)
		}
	}()

	cmd.Wait()
}

func child() {
	// fmt.Println("Child args", os.Args)

	err := syscall.Chroot("./resources")
	if err != nil {
		fmt.Printf("Error setting chroot: %v\n", err)
		os.Exit(1)
	}

	err = os.Chdir("/")
	if err != nil {
		fmt.Printf("Error chdir to root: %v\n", err)
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

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Child error: %v\n", err)
		os.Exit(1)
	}
}

// Cgroup configuration happens in the host, not the container.
func configureCgroup(container_pid int) string {
	userCgroupPath := getWritableCgroupPath()
	// fmt.Println("User dedicated cgroup: " + userCgroupPath)

	err := os.MkdirAll(userCgroupPath, 0755)
	if err != nil {
		fmt.Printf("Error creating user cgroup path: %v\n", err)
	}

	err = os.WriteFile(userCgroupPath+"/cgroup.procs", []byte(strconv.Itoa(container_pid)), 0644)
	if err != nil {
		fmt.Printf("Error assigning PID: %v\n", err)
	}

	err = os.WriteFile(userCgroupPath+"/memory.max", []byte("500000000"), 0644)
	if err != nil {
		fmt.Printf("Error set memory limit: %v\n", err)
	}

	return userCgroupPath
}

// To configure cgroup restrictions you can set them in the /sys/fs/cgroup folder,
// but to apply changes to this folder you need root access.
//
// To be able to run this app rootlessly, you can instead create cgroup files in a folder dedicated to the current user.
//
// The folder you can use can be found in /proc/self/cgroup file.
// For semplicity I copy-pasted the base format.
func getWritableCgroupPath() string {
	userUid := os.Geteuid()
	basePath := fmt.Sprintf("/sys/fs/cgroup/user.slice/user-%d.slice/user@%d.service", userUid, userUid)

	return basePath + "/mycontainer"
}

func pull() {
	//Regex [a-z0-9]+(?:[._-][a-z0-9]+)* source https://docs.docker.com/reference/api/registry/latest/
	imageName := os.Args[2]
	if !strings.Contains(imageName, "/") {
		imageName = "library/" + imageName
	}
	imageSplitTag := strings.Split(imageName, ":")
	imageName = imageSplitTag[0]
	imageTag := "latest"
	if len(imageSplitTag) > 1 && imageSplitTag[1] != "" {
		imageTag = imageSplitTag[1]
	}

	authEndpoint := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%v:pull", imageName)
	r := RegistryAuthResponse{}
	err := getJson(authEndpoint, &r)
	if err != nil {
		fmt.Printf("Error decoding auth json: %v\n", err)
	}

	// fmt.Printf("Token: %v", r.Token)
	pullEndpoint := fmt.Sprintf("https://registry-1.docker.io/v2/%v/manifests/%v", imageName, imageTag)
	req, err := http.NewRequest("GET", pullEndpoint, nil)

	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	respImage, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error pulling image: %v\n", err)
	}
	defer respImage.Body.Close()
	// dump, _ := httputil.DumpRequestOut(req, true)
	// fmt.Println(string(dump))

	if respImage.StatusCode == 404 {
		log.Fatalf("Image %v:%v not found", imageName, imageTag)
	}

	imageManifest := Manifest{}
	err = json.NewDecoder(respImage.Body).Decode(&imageManifest)
	if err != nil {
		fmt.Printf("Error decoding manifest response: %v", err)
	}

	// printStruct(imageManifest)
}

func getJson(url string, target any) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func printStruct(data any) {
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Println("Error printing struct:", err)
		return
	}
	fmt.Println(string(b))
}
