# coding-challenges
Infrastructure components and tools rewritten from scratch (like DNS or Docker) <br>

## DNS Forwarder

[Challenge Source](https://codingchallenges.fyi/challenges/challenge-dns-forwarder) <br>
[Code](./dns-forwarder/)

Open a service on :8053 <br>
Based on [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035)

Receive a dns request, forwards it to google dns and returns the result to the requester.

It also implements functions to read and write the messages in memory, but are unused. Was just an exercise to handle variables that represents single flags of DNS Message.

## Docker

[Challenge Source](https://codingchallenges.fyi/challenges/challenge-docker)

There should be a resoruces folder under the docker folder that contains an Alpine mini root filesystem.

Run it with `go run main.go run <command>` (eg: command = sh will create a shell session in the container.) 

To create a container:
1. Your cli will need to create a new namespace with the clone() syscall.
2. The process needs to replicate itself (/proc/self/exe) to run commands inside the container. It does that by recalling himself (child), with different parameters.
3. Change the root directory of the process (chroot and chdir)
4. Create a proc namespace and mount on it the namespace's proc virtual filesystem
5. Isolate the mount of proc from the host with unshare() or a private mount
6. Make the container rootless by mapping the host user to the container root. In this way the root in the container has at most the host user privileges (the user that ran the container).
7. Create cgroups files in the host to manage container resources. Max and min files to manage the resources restrictions (like memory.max) and cgroup.proc to map the container's PID to the cgroup restrictions. 


To trace the most important system calls:
```bash
strace -f -e trace=clone,setns,mount,pivot_root,chroot,openat,write -s 200 <myDockerBinary> run echo hello
```