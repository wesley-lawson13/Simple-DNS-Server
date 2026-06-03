package main

import (
	"log"
	"net"
	"os"
	"sync"
)

type cacheKey struct {
	Name  string
	Class uint16
	QType uint16
}

type queryInfo struct {
	query  message
	client net.Addr
}

type pendingQueryMap struct {
	mu     sync.Mutex
	items  map[uint16]queryInfo
	nextID uint16
}

// store assigns a unique forwarded ID to the query and returns it.
// The caller must rewrite query.hdr.Id with the returned ID before sending upstream.
func (pq *pendingQueryMap) store(info queryInfo) uint16 {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	id := pq.nextID
	pq.nextID++
	pq.items[id] = info
	return id
}

func (pq *pendingQueryMap) pop(id uint16) (queryInfo, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	info, ok := pq.items[id]
	if ok {
		delete(pq.items, id)
	}
	return info, ok
}

var NEXTSERVER net.Addr

func handleQuery(zn zone, client net.Addr, query message, localCache *cache, pending *pendingQueryMap) (message, net.Addr) {
	var resp message

	log.Printf("%v: query received\n%v\n", client, query)

	recs := zn.lookup(query.qn)

	if len(recs) != 0 {
		resp = newResponse(query, recs, true)
		log.Printf("%v: sending reply\n%v\n", client, resp)
		return resp, client
	}

	cacheRecs, found := localCache.returnRecords(query.qn)
	if found {
		resp = newResponse(query, cacheRecs, false)
		log.Printf("%v: sending cached reply\n%v\n", client, resp)
		return resp, client
	}

	log.Printf("%v: forwarding query to %v\n%v\n", client, NEXTSERVER, query)
	forwardedID := pending.store(queryInfo{query, client})
	query.hdr.Id = forwardedID

	return query, NEXTSERVER
}

func handleReply(server net.Addr, reply message, localCache *cache, pending *pendingQueryMap) (message, net.Addr) {
	log.Printf("%v: reply received\n%v\n", server, reply)

	info, ok := pending.pop(reply.hdr.Id)
	if !ok {
		log.Printf("%v: received reply for unknown query ID 0x%04x\n", server, reply.hdr.Id)
		return reply, nil
	}

	reply.hdr.Id = info.query.hdr.Id
	localCache.addRecords(reply, info.query)

	log.Printf("%v: forwarding reply to %v\n%v\n", server, info.client, reply)

	return reply, info.client
}

func handleMessage(zn zone, socket net.PacketConn, localCache *cache, pending *pendingQueryMap, client net.Addr, buf []byte) {
	localCache.update()

	msg, err := newMessage(buf)
	if err != nil {
		log.Printf("%v: %v\n", client, err)
		return
	}

	var resp message
	var addr net.Addr

	if msg.flg.QR == 0 {
		resp, addr = handleQuery(zn, client, msg, localCache, pending)
	} else {
		resp, addr = handleReply(client, msg, localCache, pending)
	}

	if addr != nil {
		if err := resp.send(socket, addr); err != nil {
			log.Printf("%v: %v\n", client, err)
			return
		}
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %v zone_file\n", os.Args[0])
	}
	zone, zerr := newZone(os.Args[1])
	if zerr != nil {
		log.Fatalln(zerr)
	}

	var nserr error
	NEXTSERVER, nserr = net.ResolveUDPAddr("udp", "127.0.0.53:53")
	if nserr != nil {
		log.Fatalln("error resolving next server address:", nserr)
	}

	pending := &pendingQueryMap{
		items: make(map[uint16]queryInfo),
	}

	localCache := &cache{
		items: make(map[cacheKey][]record),
	}

	socket, serr := net.ListenPacket("udp", "localhost:53")
	if serr != nil {
		log.Fatalln(serr)
	}
	defer socket.Close()

	log.Println("Listening on UDP port 53...")

	const MAX_DNS_LEN = 512

	for {
		var buf [MAX_DNS_LEN]byte

		n, client, err := socket.ReadFrom(buf[:])
		if err != nil {
			log.Println(err)
			continue
		}

		go handleMessage(zone, socket, localCache, pending, client, buf[:n])
	}
}
