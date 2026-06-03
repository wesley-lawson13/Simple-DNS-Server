# DNS Server

A recursive DNS forwarder implemented in Go from scratch, communicating over raw UDP sockets and parsing the DNS wire format (RFC 1035) manually — no DNS libraries used. Initially developed during my Boston College Computer Networks course, and extended to follow better practices when caching.

## How It Works

The server handles three resolution paths in priority order:

1. **Authoritative zone** — answers from a local zone file are returned immediately with the `AA` (Authoritative Answer) flag set.
2. **TTL-aware cache** — responses from upstream are stored and served from memory until their TTL expires, with the `AA` flag cleared to signal non-authoritative origin.
3. **Recursive forwarding** — queries not resolved locally are forwarded to an upstream resolver; when the reply arrives, it is cached and routed back to the original client by matching the DNS transaction ID.

## Implementation Details

- **Binary protocol parsing** — DNS messages are parsed field-by-field using `encoding/binary` with big-endian byte order. Name label compression (RFC 1035 §4.1.4) is handled via pointer offsets into the raw packet buffer.
- **UDP socket I/O** — uses `net.PacketConn` directly; each incoming packet is dispatched to a goroutine.
- **Concurrent-safe cache** — the response cache uses `sync.RWMutex` to allow parallel reads while serializing writes.
- **Concurrent-safe pending query table** — in-flight forwarded queries are tracked in a `sync.Mutex`-guarded map keyed by DNS transaction ID.
- **TTL expiration** — `cache.update()` is called on each incoming request and evicts or adjusts records whose TTL has elapsed since they were cached.
- **Record types** — A (IPv4) and CNAME records are supported; the wire encoding for each is handled separately since A records serialize as 4 raw octets and CNAME records serialize as DNS name labels.

## Zone File Format

Each line defines one resource record:

```
<name>  <ttl>  <class>  <type>  <data>
```

Example (`csci3363.zone`):

```
test1.csci3363.net  300 IN  A       1.2.3.4
test1.csci3363.net  300 IN  A       1.2.3.5
test2.csci3363.net  300 IN  CNAME   test1.csci3363.net
```

## Usage

```
go build -o dns-server .
sudo ./dns-server <zone_file>
```

Requires root (or `CAP_NET_BIND_SERVICE`) to bind port 53. The upstream resolver defaults to `127.0.0.53:53`.

Test with `dig`:

```
dig @localhost test1.csci3363.net A
dig @localhost google.com A
```
