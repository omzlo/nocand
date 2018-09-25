The NoCAN event protocol over TCP/IP
====================================

This document describes the protocol that the `nocand` server uses to communicate
with clients like `nocanc` over TCP/IP.

By implementing this protocol, users can create alternative clients to
`nocanc` or language bindings for C, Python, etc. Note that this protocol should
not be confused with the CAN bus protocol used by nodes to communicate with
each other in a NoCAN network.

A client (e.g. nocanc) initiates a communication with the `nocand` server is
as follows:

* The client connects to server and sends a _ClientHello_ event.
* The server responds with a _ServerHello_ event, including version information.
* The client sends a _ClientAuth_ event, containing a secret API key.
* The server sends a _ServerAck_ event with a "OK" status, if the API key is correct.
* The client sends a _Subscribe_ event, containing all event types it wants to receive.
* The server sends a _ServerAck_ event confirming the subscription.

After this initialisation, the server will push subscribed events to the client.
The client can send events to the sever as well at any time.

The NoCAN event manager typically listens for connections on TCP port 4242,
though this is fully configurable by the user. The protocol described in this
document should be usable without change to run over TLS/SSL, as an option
available in the future.

Event packets
-------------

Events are encoded in packets with the following structure:

|EventId  |Length       |Value
|---------|-------------|------
| 1 byte  | 1-5 byte(s) | _Length_ byte(s)

`EventId` is a number that uniquely identifies the type of event.
`Length` describes the size of the following event value.
`Value` describes data associated to the event. The structure and size of
`Value` is event dependent. In general, `Value` elements are encoded with
the most significant byte first.

### EventId

The event identifier `EventId` is represented as one byte. Currently, there are 20
different types of events:

```go
const (
    NoEvent                          EventId = 0
    ClientHelloEvent                         = 1
    ClientAuthEvent                          = 2
    ClientSubscribeEvent                     = 3
    ServerAckEvent                           = 4
    ServerHelloEvent                         = 5
    BusPowerStatusUpdateEvent                = 6
    BusPowerEvent                            = 7
    ChannelUpdateRequestEvent                = 8
    ChannelUpdateEvent                       = 9
    ChannelListRequestEvent                  = 10
    ChannelListEvent                         = 11
    NodeUpdateRequestEvent                   = 12
    NodeUpdateEvent                          = 13
    NodeListRequestEvent                     = 14
    NodeListEvent                            = 15
    NodeFirmwareUploadEvent                  = 16
    NodeFirmwareDownloadRequestEvent         = 17
    NodeFirmwareDownloadEvent                = 18
    NodeFirmwareProgressEvent                = 19
    NodeRebootRequestEvent                   = 20
)
```

These events are detailed in the following section ("Events Semantics").

### Length

The encoding of `length` is almost similar to the one used in ASN.1 DER:

* If `length<=128` then `length` is encoded as one byte.
* if `length>128` and `length<256` then `length` is encoded as the character
`0x81` followed by the value of `length` as one byte.
* if `length>=256` (i.e. 2^8) and `length<65536` then then `length` is encoded as the
character `0x82` followed by the value of `length` as two byte, most
significant byte first.
* if `length>=65536` (i.e. 2^16) and `length<16777216` then then `length` is
encoded as the character `0x83` followed by the value of `length` as three
byte, most significant byte first.
* if `length>=16777216` (i.e. 2^24) and `length<4294967296` then then `length`
is encoded as the character `0x84` followed by the value of `length` as three
byte, most significant byte first.

Examples:

* 15 is encoded as [ 0x0F ]
* 150 is encoded as [ 0x81 0x96 ]
* 1500 is encoded as [ 0x82 0x05 0xDC ]

### Value

Values are encoded as a string of bytes.
The encoding is event dependent and is described hereafter in the section
titled "Event Specification".
As a general rule, multi-byte numbers (e.g. a 16 bit integer) contained
within a structured value are represented with their most significant byte first.

Examples:

* As a 16 bit number, 258 is encoded as the string of bytes [ 0x01 0x02 ]
* As a 32 bit number, 258 is encoded as the string of bytes [ 0x00, 0x00, 0x01 0x02 ]
* As a 32 bit number, 3735928559 is encoded as [ 0xDE 0xAD, 0xBE, 0xEF ]

Event Specification
---------------

### ClientHelloEvent (1)

This event is sent at the start of session by the client. It contains no value
and has a null length.

The server should respond with a **ServerHelloEvent**.

### ClientAuthEvent (2)

A client sends an **ClientAuthEvent** to authenticate to the server, by
presenting an authentication token.
If the authentication is successful the client will be authorised to send
any type of event to the server. Unauthenticated clients can only send
**ClientAuthEvent** and **ClientHelloEvent** messages.

The content of the **ClientAuthEvent** message is simply a string representing
an authentication token:

| Authenticator |
|---------------|
| n byte(s)     |

The server will respond to this message with a **ServerAckEvent**.
If the authentication is successful, the server will set a response code to
"0". The client can then use the **ClientSubscribeEvent** message to select
what data it wants to receive from the server.

### ClientSubscribeEvent (3)

This message allows the client to specify what messages (events) it wants to
receive from the server. While the client can send any type of event, the
server will only send events that are specified through the
**ClientSubscribeEvent** message (in addition to the **ServerAckEvent**
which is always sent).

The **ClientSubscribeEvent** message value is formed as a list of EventId
codes, describing the events the client subscribes to:

| Event Id 1  | Event Id 2  | ... | Event Id N
|-------------|-------------|-----|------------
| 1 byte      | 1 byte      | ... | 1 byte

In response to this message, the server will send a **ServerAckEvent** message.

### ServerAckEvent (4)

This message indicated the success or failure of the last operation requested
by the client (e.g. authentication).

The value is a a one byte string:

| Ack code |
|----------|
| 1 byte   |

The meaning of the Ack code is as follows:

| Ack Code | Meaning
|----------|----------
| 0        | Success
| 1        | Bad or Malformed Request
| 2        | Unauthorised / missing authentication
| 3        | Not found
| 4        | General failure

### ServerHelloEvent (5)

This event is sent by the server to the client in response to a ClientHelloEvent.
The value of this event is a fixed string of 4 bytes:

| Byte 1 | Byte 2 | Byte 3 | Byte 4 |
|--------|--------|--------|--------|
| 0x45   | 0x4D   | 0x01   | 0x00   |

Byte 3 and byte 4 are used to indicate the major and minor version of then
event manager server. Currently, this is 1.0.

Once the client has received a **ServerHelloEvent**, it should proceed with a
**ClientAuthEvent**.

### BusPowerStatusUpdateEvent (6)

The server will periodically send a **BusPowerStatusUpdateEvent** message
reflecting the power status of the NoCAN network.  

### BusPowerEvent (7)

When the server sends a **BusPowerEvent** it means that the power on the NoCAN
network has been switched on or off.

This message has a one byte value:

| Power       |
|-------------|
| 1 byte (0x00 = off, 0x01 =  on) |

When a client sends a **BusPowerEvent** to the server, it means that it
requests the power to be switched on or off on the NoCAN network.
The server will respond with a **BusPowerEvent** reflecting the state of the
power on the network (it should be the same message as the client sent).

### ChannelUpdateRequestEvent (8)

A client will send a **ChannelUpdateRequestEvent** to ask the server to
report the status of a particular channel.

The message has the following structure:

| Channel Id | Channel name length | Channel name
|------------|---------------------|-----------------
| 2 bytes    | 1 bytes             | 0 to 63 bytes

The **Channel Id** represents a 16-bit channel identifier.
The **Channel Name** is the textual name of the channel.
The client can specify a channel either by id or by name.
When the **Channel Id** is not 0xFFFF, this value is used to specify a channel.
When the **Channel Id** is 0xFFFF, the name is used instead.

The server will respond to this request with a **ChannelUpdateEvent**.

### ChannelUpdateEvent (9)

A server will send a **ChannelUpdateEvent** of any change occurs on a channel
or if it receives a **ChannelUpdateRequestEvent** from a client.

The message has the following structure:

| Status | Channel Id | Channel name length | Channel name    | Channel value length | Channel Value
|--------|------------|---------------------|-----------------|----------------------|---------------
| 1 byte | 2 bytes    | 1 bytes             | 0 to 63 bytes   | 1 byte               | 0 to 63 bytes

The Status byte can have the following values:

| status | Description
|--------|------
| 0      | A channel was created.
| 1      | A channel was updated.
| 2      | A channel was destroyed.
| 3      | The requested channel does not exist (in response to a **ChannelUpdateRequestEvent** only).

The fields **Channel Id** and **Channel Name** are the same as in the
**ChannelUpdateRequestEvent**.

The field **Channel value** reprents the current content of a channel.
It is non-empty when status is 1 (channel updated);
in all other cases, **Channel value length** is 0 and **Channel value** is empty.

### ChannelListRequestEvent (10)

When the client send this message, the server will respond with a
**ChannelListEvent** containing a list of all active NoCAN channels.

This message has no value.

### ChannelListEvent (11)

The server will sent a **ChannelListEvent** in response to a
**ChannelListRequestEvent** sent by a client.

he message has the following structure:

| Channel Update event 1 | ...     | Channel Update event x
|------------------------|---------|------------
| N1 bytes               | ...     | Nx bytes

Each block in this message has the structure defined in the
**ChannelUpdateEvent** message described previously.

### NodeUpdateRequestEvent (12)

### NodeUpdateEvent (13)

### NodeListRequestEvent (14)

### NodeListEvent (15)

### NodeFirmwareUploadEvent (16)

### NodeFirmwareDownloadRequestEvent (17)

### NodeFirmwareDownloadEvent (18)

### NodeFirmwareProgressEvent (19)

### NodeRebootRequestEvent (20)
