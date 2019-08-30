package tstoremem

import (
	"errors"
	"sync"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

type memoryKeyBook struct {
	sync.RWMutex

	pks map[thread.ID]map[peer.ID]ic.PubKey
	sks map[thread.ID]map[peer.ID]ic.PrivKey
	rks map[thread.ID]map[peer.ID][]byte
	fks map[thread.ID]map[peer.ID][]byte
}

func (mkb *memoryKeyBook) getPubKey(t thread.ID, p peer.ID) (ic.PubKey, bool) {
	lmap, found := mkb.pks[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

func (mkb *memoryKeyBook) getPrivKey(t thread.ID, p peer.ID) (ic.PrivKey, bool) {
	lmap, found := mkb.sks[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

func getKey(m map[thread.ID]map[peer.ID][]byte, t thread.ID, p peer.ID) ([]byte, bool) {
	lmap, found := m[t]
	if lmap == nil {
		return nil, found
	}
	hmap, found := lmap[p]
	return hmap, found
}

func NewLogKeyBook() tstore.LogKeyBook {
	return &memoryKeyBook{
		pks: map[thread.ID]map[peer.ID]ic.PubKey{},
		sks: map[thread.ID]map[peer.ID]ic.PrivKey{},
		rks: map[thread.ID]map[peer.ID][]byte{},
		fks: map[thread.ID]map[peer.ID][]byte{},
	}
}

func (mkb *memoryKeyBook) LogsWithKeys(t thread.ID) peer.IDSlice {
	mkb.RLock()
	ps := make(map[peer.ID]struct{})
	if mkb.pks[t] != nil {
		for p := range mkb.pks[t] {
			ps[p] = struct{}{}
		}
	}
	if mkb.sks[t] != nil {
		for p := range mkb.sks[t] {
			ps[p] = struct{}{}
		}
	}
	mkb.RUnlock()
	var pids peer.IDSlice
	for p := range ps {
		pids = append(pids, p)
	}
	return pids
}

func (mkb *memoryKeyBook) ThreadsFromKeys() thread.IDSlice {
	mkb.RLock()
	ts := make(map[thread.ID]struct{})
	for t := range mkb.pks {
		ts[t] = struct{}{}
	}
	for t := range mkb.sks {
		ts[t] = struct{}{}
	}
	mkb.RUnlock()
	var tids thread.IDSlice
	for t := range ts {
		tids = append(tids, t)
	}
	return tids
}

func (mkb *memoryKeyBook) LogPubKey(t thread.ID, p peer.ID) ic.PubKey {
	mkb.RLock()
	pk, _ := mkb.getPubKey(t, p)
	mkb.RUnlock()
	if pk != nil {
		return pk
	}
	pk, err := p.ExtractPublicKey()
	if err == nil {
		mkb.Lock()
		if mkb.pks[t] == nil {
			mkb.pks[t] = make(map[peer.ID]ic.PubKey, 1)
		}
		mkb.pks[t][p] = pk
		mkb.Unlock()
	}
	return pk
}

func (mkb *memoryKeyBook) AddLogPubKey(t thread.ID, p peer.ID, pk ic.PubKey) error {
	// check it's correct first
	if !p.MatchesPublicKey(pk) {
		return errors.New("ID does not match PublicKey")
	}

	mkb.Lock()
	if mkb.pks[t] == nil {
		mkb.pks[t] = make(map[peer.ID]ic.PubKey, 1)
	}
	mkb.pks[t][p] = pk
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) LogPrivKey(t thread.ID, p peer.ID) ic.PrivKey {
	mkb.RLock()
	sk, _ := mkb.getPrivKey(t, p)
	mkb.RUnlock()
	return sk
}

func (mkb *memoryKeyBook) AddLogPrivKey(t thread.ID, p peer.ID, sk ic.PrivKey) error {
	if sk == nil {
		return errors.New("sk is nil (PrivKey)")
	}

	// check it's correct first
	if !p.MatchesPrivateKey(sk) {
		return errors.New("ID does not match PrivateKey")
	}

	mkb.Lock()
	if mkb.sks[t] == nil {
		mkb.sks[t] = make(map[peer.ID]ic.PrivKey, 1)
	}
	mkb.sks[t][p] = sk
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) LogReadKey(t thread.ID, p peer.ID) []byte {
	mkb.RLock()
	key, _ := getKey(mkb.rks, t, p)
	mkb.RUnlock()
	return key
}

func (mkb *memoryKeyBook) AddLogReadKey(t thread.ID, p peer.ID, key []byte) error {
	if key == nil {
		return errors.New("key is nil (ReadKey)")
	}

	mkb.Lock()
	if mkb.rks[t] == nil {
		mkb.rks[t] = make(map[peer.ID][]byte, 1)
	}
	mkb.rks[t][p] = key
	mkb.Unlock()
	return nil
}

func (mkb *memoryKeyBook) LogFollowKey(t thread.ID, p peer.ID) []byte {
	mkb.RLock()
	key, _ := getKey(mkb.fks, t, p)
	mkb.RUnlock()
	return key
}

func (mkb *memoryKeyBook) AddLogFollowKey(t thread.ID, p peer.ID, key []byte) error {
	if key == nil {
		return errors.New("key is nil (FollowKey)")
	}

	mkb.Lock()
	if mkb.fks[t] == nil {
		mkb.fks[t] = make(map[peer.ID][]byte, 1)
	}
	mkb.fks[t][p] = key
	mkb.Unlock()
	return nil
}
