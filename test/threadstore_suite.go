package test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/thread"
	tstore "github.com/textileio/go-textile-core/threadstore"
)

var threadstoreSuite = map[string]func(tstore.Threadstore) func(*testing.T){
	"AddrStream":              testAddrStream,
	"GetStreamBeforeLogAdded": testGetStreamBeforeLogAdded,
	"AddStreamDuplicates":     testAddrStreamDuplicates,
	"BasicThreadstore":        testBasicThreadstore,
	"Metadata":                testMetadata,
}

type ThreadstoreFactory func() (tstore.Threadstore, func())

func ThreadstoreTest(t *testing.T, factory ThreadstoreFactory) {
	for name, test := range threadstoreSuite {
		// Create a new threadstore.
		ps, closeFunc := factory()

		// Run the test.
		t.Run(name, test(ps))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testAddrStream(ts tstore.Threadstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		addrs, pid := getAddrs(t, 100), peer.ID("testlog")
		ts.AddLogAddrs(tid, pid, addrs[:10], time.Hour)

		ctx, cancel := context.WithCancel(context.Background())
		addrch := ts.LogAddrStream(ctx, tid, pid)

		// while that subscription is active, publish ten more addrs
		// this tests that it doesnt hang
		for i := 10; i < 20; i++ {
			ts.AddLogAddr(tid, pid, addrs[i], time.Hour)
		}

		// now receive them (without hanging)
		timeout := time.After(time.Second * 10)
		for i := 0; i < 20; i++ {
			select {
			case <-addrch:
			case <-timeout:
				t.Fatal("timed out")
			}
		}

		// start a second stream
		ctx2, cancel2 := context.WithCancel(context.Background())
		addrch2 := ts.LogAddrStream(ctx2, tid, pid)

		done := make(chan struct{})
		go func() {
			defer close(done)
			// now send the rest of the addresses
			for _, a := range addrs[20:80] {
				ts.AddLogAddr(tid, pid, a, time.Hour)
			}
		}()

		// receive some concurrently with the goroutine
		timeout = time.After(time.Second * 10)
		for i := 0; i < 40; i++ {
			select {
			case <-addrch:
			case <-timeout:
			}
		}

		<-done

		// receive some more after waiting for that goroutine to complete
		timeout = time.After(time.Second * 10)
		for i := 0; i < 20; i++ {
			select {
			case <-addrch:
			case <-timeout:
			}
		}

		// now cancel it
		cancel()

		// now check the *second* subscription. We should see 80 addresses.
		for i := 0; i < 80; i++ {
			<-addrch2
		}

		cancel2()

		// and add a few more addresses it doesnt hang afterwards
		for _, a := range addrs[80:] {
			ts.AddLogAddr(tid, pid, a, time.Hour)
		}
	}
}

func testGetStreamBeforeLogAdded(ts tstore.Threadstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		addrs, pid := getAddrs(t, 10), peer.ID("testlog")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ach := ts.LogAddrStream(ctx, tid, pid)
		for i := 0; i < 10; i++ {
			ts.AddLogAddr(tid, pid, addrs[i], time.Hour)
		}

		received := make(map[string]bool)
		var count int

		for i := 0; i < 10; i++ {
			a, ok := <-ach
			if !ok {
				t.Fatal("channel shouldnt be closed yet")
			}
			if a == nil {
				t.Fatal("got a nil address, thats weird")
			}
			count++
			if received[a.String()] {
				t.Fatal("received duplicate address")
			}
			received[a.String()] = true
		}

		select {
		case <-ach:
			t.Fatal("shouldnt have received any more addresses")
		default:
		}

		if count != 10 {
			t.Fatal("should have received exactly ten addresses, got ", count)
		}

		for _, a := range addrs {
			if !received[a.String()] {
				t.Log(received)
				t.Fatalf("expected to receive address %s but didnt", a)
			}
		}
	}
}

func testAddrStreamDuplicates(ts tstore.Threadstore) func(t *testing.T) {
	return func(t *testing.T) {
		tid := thread.NewIDV1(thread.Raw, 24)

		addrs, pid := getAddrs(t, 10), peer.ID("testlog")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ach := ts.LogAddrStream(ctx, tid, pid)
		go func() {
			for i := 0; i < 10; i++ {
				ts.AddLogAddr(tid, pid, addrs[i], time.Hour)
				ts.AddLogAddr(tid, pid, addrs[rand.Intn(10)], time.Hour)
			}

			// make sure that all addresses get processed before context is cancelled
			time.Sleep(time.Millisecond * 50)
			cancel()
		}()

		received := make(map[string]bool)
		var count int
		for a := range ach {
			if a == nil {
				t.Fatal("got a nil address, thats weird")
			}
			count++
			if received[a.String()] {
				t.Fatal("received duplicate address")
			}
			received[a.String()] = true
		}

		if count != 10 {
			t.Fatal("should have received exactly ten addresses")
		}
	}
}

func testBasicThreadstore(ts tstore.Threadstore) func(t *testing.T) {
	return func(t *testing.T) {
		tids := make([]thread.ID, 0)
		addrs := getAddrs(t, 10)

		for _, a := range addrs {
			tid := thread.NewIDV1(thread.Raw, 24)
			tids = append(tids, tid)
			priv, _, _ := crypto.GenerateKeyPair(crypto.RSA, 512)
			p, _ := peer.IDFromPrivateKey(priv)
			ts.AddLogAddr(tid, p, a, pstore.PermanentAddrTTL)
		}

		threads := ts.Threads()
		if len(threads) != 10 {
			t.Fatal("expected ten threads, got", len(threads))
		}

		info := ts.ThreadInfo(tids[0])
		tsAddrs := ts.LogAddrs(info.ID, info.Logs[0])
		if !tsAddrs[0].Equal(addrs[0]) {
			t.Fatal("stored wrong address")
		}
	}
}

func testMetadata(ts tstore.Threadstore) func(t *testing.T) {
	return func(t *testing.T) {
		tids := make([]thread.ID, 10)
		for i := range tids {
			tids[i] = thread.NewIDV1(thread.Raw, 24)
		}
		for _, p := range tids {
			if err := ts.PutMeta(p, "AgentVersion", "string"); err != nil {
				t.Errorf("failed to put %q: %s", "AgentVersion", err)
			}
			if err := ts.PutMeta(p, "bar", 1); err != nil {
				t.Errorf("failed to put %q: %s", "bar", err)
			}
		}
		for _, p := range tids {
			v, err := ts.GetMeta(p, "AgentVersion")
			if err != nil {
				t.Errorf("failed to find %q: %s", "AgentVersion", err)
				continue
			}
			if v != "string" {
				t.Errorf("expected %q, got %q", "string", p)
				continue
			}

			v, err = ts.GetMeta(p, "bar")
			if err != nil {
				t.Errorf("failed to find %q: %s", "bar", err)
				continue
			}
			if v != 1 {
				t.Errorf("expected %q, got %v", 1, v)
				continue
			}
		}
	}
}

func getAddrs(t *testing.T, n int) []ma.Multiaddr {
	var addrs []ma.Multiaddr
	for i := 0; i < n; i++ {
		a, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", i))
		if err != nil {
			t.Fatal(err)
		}

		addrs = append(addrs, a)
	}
	return addrs
}
