package main

import (
	"log"
	"net"
	"os"
)

type cacheKey struct {
    Name string
    Class uint16
    QType uint16
}

type cache struct {
    items map[cacheKey][]record
}

type queryInfo struct {
    query message
    client net.Addr
}

var pendingQueries map[uint16]queryInfo

var (
    // this is the address of the DNS server we will contact next
    // it's initialized once and not changed, so effectively constant
    NEXTSERVER net.Addr
)

// handle one query
func handleQuery(zn zone, client net.Addr, query message, localCache *cache) (message, net.Addr) {
    
    var resp message

    // print the message nicely
    log.Printf("%v: query received\n%v\n", client, query)

    // lookup the question in the zone
    recs := zn.lookup(query.qn)

    // if we found any records, send a response
    if len(recs) != 0 {
        // build the response message
        resp = newResponse(query, recs, true)

        // print the response
        log.Printf("%v: sending reply\n%v\n", client, resp)

        return resp, client
    }

    // TODO: check the cache and send a response if cached
    cacheRecs, found := localCache.returnRecords(query.qn)
    if found {
        resp = newResponse(query, cacheRecs, false)

        log.Printf("%v: sending cached reply\n%v\n", client, resp)

        return resp, client
    }

    // 1) if we didn't find the answer in our zone or cache
    //       then send the query on to the next server
    //       2) store the client's address so we can find it later
    log.Printf("%v: forwarding query to %v\n%v\n", client, NEXTSERVER, query)

    pendingQueries[query.hdr.Id] = queryInfo{query, client}

    return query, NEXTSERVER
}

// handle one reply
func handleReply(server net.Addr, reply message, localCache *cache) (message, net.Addr) {
    // print the message nicely
    log.Printf("%v: reply received\n%v\n", server, reply)

    // clean up the pending queries 
    client, query := pendingQueries[reply.hdr.Id].client, pendingQueries[reply.hdr.Id].query
    delete(pendingQueries, reply.hdr.Id)

    // add the records in this reply to the cache
    localCache.addRecords(reply, query)

    // forward the reply back to the original client
    log.Printf("%v: forwarding reply to %v\n%v\n", server, client, reply)

    return reply, client
}

// handle one DNS message
func handleMessage(zn zone, socket net.PacketConn, localCache *cache, client net.Addr, buf []byte) {
    // update the cache, removing any records with expired TTLs
    localCache.update()

    // parse the message
    msg, err := newMessage(buf)
    if err != nil {
        log.Printf("%v: %v\n", client, err)
        return
    }

    var resp message
    var addr net.Addr 

    if msg.flg.QR == 0 {
        // handle queries
        resp,addr = handleQuery(zn, client, msg, localCache)

    } else {
        // handle replies
        resp,addr = handleReply(client, msg, localCache)
    }

    if addr != nil {
        // send the response message
        if err := resp.send(socket, addr); err != nil {
            log.Printf("%v: %v\n", client, err)
            return
        }
    }
}

func main() {
    // read the zone file
    if len(os.Args) != 2 {
        log.Fatalf("usage: %v zone_file\n", os.Args[0])
    }
    zone, zerr := newZone(os.Args[1])
    if zerr != nil {
        log.Fatalln(zerr)
    }

    // get the next server address set up
    var nserr error
    NEXTSERVER, nserr = net.ResolveUDPAddr("udp", "127.0.0.53:53")
	if nserr != nil {
		log.Println("Error resolving next server address")
		return
	}

    // initialize the pendies queries structure
    pendingQueries = make(map[uint16]queryInfo)

    // initialize the cache
    localCache := &cache{
        items: make(map[cacheKey][]record),
    }

    // listen for incoming UDP packets on port 53
    socket, serr := net.ListenPacket("udp","localhost:53") 
    if serr != nil {
        log.Fatalln(serr)
    }
    
    // close the socket when this function returns
    defer socket.Close()

    log.Println("Listening on UDP port 53...")

    // max packet length for DNS is 512 bytes
    const MAX_DNS_LEN = 512

    for {
        // make a byte buffer to hold the incoming packet data
        var buf [MAX_DNS_LEN]byte

        // block until a new packet arrives, put it into buf
        n, client, err := socket.ReadFrom(buf[:])
        if err != nil {
            log.Println(err)
            continue
        }

        // spawn a new thread to handle this packet
        go handleMessage(zone, socket, localCache, client, buf[:n])
    }
}
