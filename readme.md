## Remote VM

Remote IPC system using a bytecode VM.

This could be an alternative to SQL, GraphQL, SSH, REST



### Test

For test you'll need a tls certificate: 

```openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes```

### Design
- Support some kind of authentication.
- Forwarding
- Client events

### Demo

Demonstrate the capabilities by:

#### User Space OS

Design a system where a client app can draw pictures on a host application framework. Each client app is a child process clicks and other events sent to the child process are events going the other way.

#### File Server

Make a system uploading, downloading, listing files