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