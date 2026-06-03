package main

import (
	"sync"
	"time"
)

type cache struct {
	mu    sync.RWMutex
	items map[cacheKey][]record
}

func makeKey(q question) cacheKey {
	var key cacheKey
	key.Class, key.QType, key.Name = q.Class, q.QType, q.Name
	return key
}

func (ch *cache) addRecords(reply, query message) {
	if len(reply.ans) == 0 {
		return
	}

	key := makeKey(query.qn)
	recs := make([]record, len(reply.ans))
	copy(recs, reply.ans)

	for i := range recs {
		recs[i].AddedAt = time.Now()
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.items[key] = recs
}

func (ch *cache) returnRecords(query question) ([]record, bool) {
	key := makeKey(query)

	ch.mu.RLock()
	recs, ok := ch.items[key]
	ch.mu.RUnlock()

	if !ok || len(recs) == 0 {
		return nil, false
	}

	ret := make([]record, len(recs))
	copy(ret, recs)

	return ret, true
}

func (rec *record) expired(now time.Time) bool {
	elapsed := uint32(now.Sub(rec.AddedAt).Seconds())
	return elapsed >= rec.TTL
}

func (ch *cache) update() {
	now := time.Now()

	ch.mu.Lock()
	defer ch.mu.Unlock()

	for key, item := range ch.items {
		persistentRecs := make([]record, 0, len(item))

		for i := range item {
			rec := item[i]
			if !rec.expired(now) {
				elapsed := uint32(now.Sub(rec.AddedAt).Seconds())
				rec.TTL -= elapsed
				rec.AddedAt = now
				persistentRecs = append(persistentRecs, rec)
			}
		}

		if len(persistentRecs) == 0 {
			delete(ch.items, key)
		} else {
			ch.items[key] = persistentRecs
		}
	}
}
