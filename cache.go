package main

import (
	"time"
)

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

	ch.items[key] = recs
}

func (ch *cache) returnRecords(query question) ([]record, bool) {

	key := makeKey(query)

	recs, ok := ch.items[key]

	if !ok || len(recs) == 0 {
		return nil, false
	}

	ret := make([]record, len(recs))
	copy(ret, recs)

	return ret, true
}

func (rec *record) expired(now time.Time) bool {

    elapsed := uint32(now.Sub(rec.AddedAt).Seconds())
    if elapsed >= rec.TTL {
        return true // expired
    }

    return false 
}

func (ch *cache) update() {

    now := time.Now()
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