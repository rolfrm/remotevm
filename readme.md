## Whats

Peer to peer communication facilitator using nothing but websockets.

- Clients in the network are connected using some nodes.
- Clients subscribe to the node using websockets.
- Nodes connect with eachother using websockets.
- Direct tcp-like communication.
- Order of packages are guaranteed, unless the connection is broken.
- Packages may get retransmitted, but the clients wont know about it.
- There is no persistence in the network.
- There is no state in the network except for the connections themselves.
- Nodes can authenticate using tokens
- Tokens can give access to services.



### Clients A wants to connect to B.

Connect to the address by calling server/B. This will create a stream to and from that server. The stream might jump through multiple steps.

### Client wants to present it self as a service.

- Connect to the server. 
- The server registers the websocket and keeps track of a special control stream.

When a client connects to the service
- The server asks the service to open a new websocket for communication.
- The server transfers all information between the service and the client.


## Version 1

There is one node in the system. Services connect to that node.