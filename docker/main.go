package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type RegistryAuthResponse struct {
	Token string
}

const (
	ANSIColorBlue  = "\033[34m"
	ANSIColorReset = "\033[0m"
	ANSIBold       = "\033[1m"
)

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

var (
	httpClient = &http.Client{}
	verbose    bool
	remove     bool
)

func main() {
	flag.BoolVar(&verbose, "v", false, "enable verbose logging")
	flag.BoolVar(&remove, "rm", false, "delete the container after exiting")
	flag.Parse()
	argsWithoutFlags := flag.Args()

	logverbose("Parent args: %v", os.Args)

	if os.Getenv("VERBOSE") == "true" {
		verbose = true
	}

	if os.Getenv("IS_CHILD_PROCESS") == "true" {
		child(argsWithoutFlags)
		return
	}

	command := argsWithoutFlags[0]
	switch command {
	case "run":
		pull(argsWithoutFlags)
		run(argsWithoutFlags[2:])
	case "pull":
		if len(argsWithoutFlags) < 2 {
			log.Fatalf("Received only %d param. Expected 2", len(argsWithoutFlags))
		}
		pull(argsWithoutFlags)
	default:
		fmt.Printf("Unknown command: %v\n", argsWithoutFlags[0])
	}

	if remove {
		myContainerFolder := myContainerFolder()
		err := os.RemoveAll(myContainerFolder)
		if err != nil {
			log.Fatalf("Error deleting mycontainer folder at path %v: %v", myContainerFolder, err)
		}
	}
}

// The run method clones himself into a new child process by reexecuting the application again via
// /proc/self/exe, and setting the [IS_CHILD_PROCESS] env variable.
//
// The child process is then put in new namespaces and has cgroup configured.
func run(args []string) {
	cmd := exec.Command("/proc/self/exe", args...)
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
	if verbose {
		cmd.Env = append(cmd.Env, "VERBOSE=true")
	}

	err := cmd.Start()
	if err != nil {
		fmt.Printf("ERROR: The command failed to run: %v\n", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
	}

	configureNetwork(cmd.Process.Pid)

	userCgroupPath := configureCgroup(cmd.Process.Pid)
	defer func() {
		if err := os.Remove(userCgroupPath); err != nil {
			fmt.Printf("Error user Cgroup path removal: %v\n", err)
		}
	}()

	cmd.Wait()
}

func child(args []string) {
	logverbose("Child args:%v", args)

	myContainerFolder := myContainerFolder()
	err := syscall.Chroot(myContainerFolder)
	logverbose("mycontainer folder is: %v", myContainerFolder)
	if err != nil {
		fmt.Printf("Error setting chroot: %v\n", err)
		os.Exit(1)
	}

	err = os.Chdir("/")
	if err != nil {
		fmt.Printf("Error chdir to root: %v\n", err)
		os.Exit(1)
	}

	os.Mkdir("/proc", 0755)
	err = syscall.Mount("proc", "/proc", "proc", 0, "")
	if err != nil {
		fmt.Printf("Error proc mount: %v\n", err)
		os.Exit(1)
	}

	os.Mkdir("/sys", 0755)
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

	cmd := exec.Command(args[0], args[1:]...)
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
	logverbose("User dedicated cgroup: %v", userCgroupPath)

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

func pull(args []string) {
	//Regex [a-z0-9]+(?:[._-][a-z0-9]+)* source https://docs.docker.com/reference/api/registry/latest/
	imageName := args[1]
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

	pullEndpoint := fmt.Sprintf("https://registry-1.docker.io/v2/%v/manifests/%v", imageName, imageTag)
	req, err := http.NewRequest("GET", pullEndpoint, nil)

	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	manifestsList := pullManifest(r.Token, imageName, imageTag)
	logverbose("Manifest list:\n%v", printStruct(manifestsList))

	digest := digestByArch(manifestsList, runtime.GOARCH, runtime.GOOS)
	if digest == "" {
		log.Fatal("No compatible image was found with current distribution")
	}

	manifest := pullManifest(r.Token, imageName, digest)

	logverbose("Manifest:\n%v", printStruct(manifest))

	for i := range manifest.Layers {
		layerZip := pullLayer(r.Token, imageName, manifest.Layers[i].Digest)
		fmt.Printf("Extracting layer %v\n", layerZip)
		err = extractLayer(layerZip, myContainerFolder())
		if err != nil {
			fmt.Printf("Error while extracting layer: %v\n", err)
		}
		err = os.Remove(layerZip)
		if err != nil {
			fmt.Printf("Error while deleting layer: %v\n", err)
		}
	}
}

func getJson(url string, target any) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func printStruct(data any) string {
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Println("Error printing struct:", err)
	}
	return fmt.Sprint(string(b))
}

func digestByArch(list Manifest, arch string, os string) string {
	for i := range list.Manifests {
		if list.Manifests[i].Platform.Architecture == arch && list.Manifests[i].Platform.OS == os {
			return list.Manifests[i].Digest
		}
	}

	return ""
}

func pullManifest(token string, imageName string, reference string) Manifest {
	pullEndpoint := fmt.Sprintf("https://registry-1.docker.io/v2/%v/manifests/%v", imageName, reference)
	req, _ := http.NewRequest("GET", pullEndpoint, nil)

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	respManifest, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error pulling image: %v\n", err)
	}
	defer respManifest.Body.Close()

	if verbose {
		dump, _ := httputil.DumpRequestOut(req, true)
		logverbose("HTTP pull request:\n%v", string(dump))
	}

	if respManifest.StatusCode == 404 {
		log.Fatalf("Image %v:%v not found", imageName, reference)
	}

	manifestsList := Manifest{}
	err = json.NewDecoder(respManifest.Body).Decode(&manifestsList)
	if err != nil {
		fmt.Printf("Error decoding manifest response: %v", err)
	}

	return manifestsList
}

func pullLayer(token string, imageName string, reference string) string {
	pullEndpoint := fmt.Sprintf("https://registry-1.docker.io/v2/%v/blobs/%v", imageName, reference)
	req, _ := http.NewRequest("GET", pullEndpoint, nil)

	req.Header.Set("Authorization", "Bearer "+token)

	r, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error downloading layer: %v", err)
	}
	defer r.Body.Close()

	fileName := filepath.Join(os.TempDir(), strings.TrimPrefix(reference, "sha256:")+".tar")
	out, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer out.Close()

	fmt.Printf("Downloading file: %s...\n", fileName)
	_, err = io.Copy(out, r.Body)
	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

	return fileName
}

func extractLayer(fileName string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	cmd := exec.Command("tar", "-xf", fileName, "-C", targetDir)
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func logverbose(format string, v ...any) {
	if verbose {
		fmt.Printf(ANSIBold+ANSIColorBlue+"[LOG]"+ANSIColorReset+" "+format+"\n", v...)
	}
}

func myContainerFolder() string {
	myContainerFolder, _ := os.UserConfigDir()
	myContainerFolder += "/mydocker/container"
	err := os.MkdirAll(myContainerFolder, 0750)
	if err != nil {
		log.Fatalf("Error while creating mycontainer folder at '%v': %v", myContainerFolder, err)
	}

	return myContainerFolder
}

// Implement network bridge with slirp4netns
// Just follow instructions on the github page
// https://github.com/rootless-containers/slirp4netns?tab=readme-ov-file#usage
func configureNetwork(pid int) {
	netCmd := exec.Command("slirp4netns",
		"--configure",
		"--mtu=65520",
		"--disable-host-loopback",
		strconv.Itoa(pid),
		"tap0",
	)

	err := netCmd.Start()
	if err != nil {
		fmt.Printf("Networking failed: %v. Is slirp4netns installed?\n", err)
	}
}
