package util

import (
	"crypto/rand"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	sym "github.com/textileio/go-textile-core/crypto/symmetric"
	"github.com/textileio/go-textile-core/thread"
	tserv "github.com/textileio/go-textile-core/threadservice"
)

// CreateThread creates a new set of keys.
func CreateThread(t tserv.Threadservice, id thread.ID) (info thread.Info, err error) {
	info.ID = id
	info.FollowKey, err = sym.CreateKey()
	if err != nil {
		return
	}
	info.ReadKey, err = sym.CreateKey()
	if err != nil {
		return
	}
	err = t.Store().AddThread(info)
	return
}

// CreateLog creates a new log with the given peer as host.
func CreateLog(host peer.ID) (info thread.LogInfo, err error) {
	sk, pk, err := ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return
	}
	pro := ma.ProtocolWithCode(ma.P_P2P).Name
	addr, err := ma.NewMultiaddr("/" + pro + "/" + host.String())
	if err != nil {
		return
	}
	return thread.LogInfo{
		ID:      id,
		PubKey:  pk,
		PrivKey: sk,
		Addrs:   []ma.Multiaddr{addr},
	}, nil
}

// GetOrCreateLog returns the log with the given thread and log id.
// If no log exists, a new one is created under the given thread.
func GetOrCreateLog(t tserv.Threadservice, id thread.ID, lid peer.ID) (info thread.LogInfo, err error) {
	info, err = t.Store().LogInfo(id, lid)
	if err != nil {
		return
	}
	if info.PubKey != nil {
		return
	}
	info, err = CreateLog(t.Host().ID())
	if err != nil {
		return
	}
	err = t.Store().AddLog(id, info)
	return
}

// GetOwnLoad returns the log owned by the host under the given thread.
func GetOwnLog(t tserv.Threadservice, id thread.ID) (info thread.LogInfo, err error) {
	logs, err := t.Store().LogsWithKeys(id)
	if err != nil {
		return
	}
	for _, lid := range logs {
		sk, err := t.Store().PrivKey(id, lid)
		if err != nil {
			return info, err
		}
		if sk != nil {
			return t.Store().LogInfo(id, lid)
		}
	}
	return info, nil
}

// GetOrCreateOwnLoad returns the log owned by the host under the given thread.
// If no log exists, a new one is created under the given thread.
func GetOrCreateOwnLog(t tserv.Threadservice, id thread.ID) (info thread.LogInfo, err error) {
	info, err = GetOwnLog(t, id)
	if err != nil {
		return info, err
	}
	if info.PubKey != nil {
		return
	}
	info, err = CreateLog(t.Host().ID())
	if err != nil {
		return
	}
	err = t.Store().AddLog(id, info)
	return info, err
}