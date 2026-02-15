# coding-challenges
Infrastructure components and tools (like DNS or Docker) written from scratch. <br>
[Source](https://codingchallenges.fyi/challenges/intro)

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
To create a container:
1. Your cli will need to create a new namespace with the clone() syscall. In this case an UTS namespace.
2. The process needs to replicate itself (/proc/self/exe) to run commands inside the container. It does that by recalling himself (child), with different parameters.
3. Change the root directory of the process (chroot and chdir)