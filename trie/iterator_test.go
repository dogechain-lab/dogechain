// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package trie

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/helper/kvdb/memorydb"
	"github.com/dogechain-lab/dogechain/helper/rawdb"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
)

func TestEmptyIterator(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase(), hclog.NewNullLogger()))
	iter := trie.NodeIterator(nil)

	seen := make(map[string]struct{})
	for iter.Next(true) {
		seen[string(iter.Path())] = struct{}{}
	}
	if len(seen) != 0 {
		t.Fatal("Unexpected trie node iterated")
	}
}

func TestIterator(t *testing.T) {
	logger := hclog.NewNullLogger()
	db := NewDatabase(rawdb.NewMemoryDatabase(), logger)
	trie := NewEmpty(db)
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"dog", "puppy"},
		{"somethingveryoddindeedthis is", "myothernodedata"},
	}
	all := make(map[string]string)
	for _, val := range vals {
		all[val.k] = val.v
		trie.Update([]byte(val.k), []byte(val.v))
	}
	root, nodes, err := trie.Commit(false)
	if err != nil {
		t.Fatalf("Failed to commit trie %v", err)
	}
	db.Update(NewWithNodeSet(nodes))

	trie, _ = New(TrieID(root), db, logger)
	found := make(map[string]string)
	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		found[string(it.Key)] = string(it.Value)
	}

	for k, v := range all {
		if found[k] != v {
			t.Errorf("iterator value mismatch for %s: got %q want %q", k, found[k], v)
		}
	}
}

type kv struct {
	k, v []byte
	t    bool
}

func TestIteratorLargeData(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase(), hclog.NewNullLogger()))
	vals := make(map[string]*kv)

	for i := byte(0); i < 255; i++ {
		value := &kv{types.LeftPadBytes([]byte{i}, 32), []byte{i}, false}
		value2 := &kv{types.LeftPadBytes([]byte{10, i}, 32), []byte{i}, false}
		trie.Update(value.k, value.v)
		trie.Update(value2.k, value2.v)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}

	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		vals[string(it.Key)].t = true
	}

	var untouched []*kv
	for _, value := range vals {
		if !value.t {
			untouched = append(untouched, value)
		}
	}

	if len(untouched) > 0 {
		t.Errorf("Missed %d nodes", len(untouched))
		for _, value := range untouched {
			t.Error(value)
		}
	}
}

// Tests that the node iterator indeed walks over the entire database contents.
func TestNodeIteratorCoverage(t *testing.T) {
	// Create some arbitrary test trie to iterate
	db, trie, _ := makeTestTrie()

	// Gather all the node hashes found by the iterator
	hashes := make(map[types.Hash]struct{})
	for it := trie.NodeIterator(nil); it.Next(true); {
		if it.Hash() != (types.Hash{}) {
			hashes[it.Hash()] = struct{}{}
		}
	}
	// Cross check the hashes and the database itself
	for hash := range hashes {
		if _, err := db.Node(hash); err != nil {
			t.Errorf("failed to retrieve reported node %x: %v", hash, err)
		}
	}
	for hash, obj := range db.dirties {
		if obj != nil && hash != (types.Hash{}) {
			if _, ok := hashes[hash]; !ok {
				t.Errorf("state entry not reported %x", hash)
			}
		}
	}
	it := db.diskdb.NewIterator(nil, nil)
	for it.Next() {
		key := it.Key()
		if _, ok := hashes[types.BytesToHash(key)]; !ok {
			t.Errorf("state entry not reported %x", key)
		}
	}
	it.Release()
}

type kvs struct{ k, v string }

var testdata1 = []kvs{
	{"barb", "ba"},
	{"bard", "bc"},
	{"bars", "bb"},
	{"bar", "b"},
	{"fab", "z"},
	{"food", "ab"},
	{"foos", "aa"},
	{"foo", "a"},
}

var testdata2 = []kvs{
	{"aardvark", "c"},
	{"bar", "b"},
	{"barb", "bd"},
	{"bars", "be"},
	{"fab", "z"},
	{"foo", "a"},
	{"foos", "aa"},
	{"food", "ab"},
	{"jars", "d"},
}

func TestIteratorSeek(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase(), hclog.NewNullLogger()))
	for _, val := range testdata1 {
		trie.Update([]byte(val.k), []byte(val.v))
	}

	// Seek to the middle.
	it := NewIterator(trie.NodeIterator([]byte("fab")))
	if err := checkIteratorOrder(testdata1[4:], it); err != nil {
		t.Fatal(err)
	}

	// Seek to a non-existent key.
	it = NewIterator(trie.NodeIterator([]byte("barc")))
	if err := checkIteratorOrder(testdata1[1:], it); err != nil {
		t.Fatal(err)
	}

	// Seek beyond the end.
	it = NewIterator(trie.NodeIterator([]byte("z")))
	if err := checkIteratorOrder(nil, it); err != nil {
		t.Fatal(err)
	}
}

func checkIteratorOrder(want []kvs, it *Iterator) error {
	for it.Next() {
		if len(want) == 0 {
			return fmt.Errorf("didn't expect any more values, got key %q", it.Key)
		}
		if !bytes.Equal(it.Key, []byte(want[0].k)) {
			return fmt.Errorf("wrong key: got %q, want %q", it.Key, want[0].k)
		}
		want = want[1:]
	}
	if len(want) > 0 {
		return fmt.Errorf("iterator ended early, want key %q", want[0])
	}
	return nil
}

func TestDifferenceIterator(t *testing.T) {
	logger := hclog.NewNullLogger()
	dba := NewDatabase(rawdb.NewMemoryDatabase(), logger)
	triea := NewEmpty(dba)
	for _, val := range testdata1 {
		triea.Update([]byte(val.k), []byte(val.v))
	}
	rootA, nodesA, _ := triea.Commit(false)
	dba.Update(NewWithNodeSet(nodesA))
	triea, _ = New(TrieID(rootA), dba, logger)

	dbb := NewDatabase(rawdb.NewMemoryDatabase(), hclog.NewNullLogger())
	trieb := NewEmpty(dbb)
	for _, val := range testdata2 {
		trieb.Update([]byte(val.k), []byte(val.v))
	}
	rootB, nodesB, _ := trieb.Commit(false)
	dbb.Update(NewWithNodeSet(nodesB))
	trieb, _ = New(TrieID(rootB), dbb, logger)

	found := make(map[string]string)
	di, _ := NewDifferenceIterator(triea.NodeIterator(nil), trieb.NodeIterator(nil))
	it := NewIterator(di)
	for it.Next() {
		found[string(it.Key)] = string(it.Value)
	}

	all := []struct{ k, v string }{
		{"aardvark", "c"},
		{"barb", "bd"},
		{"bars", "be"},
		{"jars", "d"},
	}
	for _, item := range all {
		if found[item.k] != item.v {
			t.Errorf("iterator value mismatch for %s: got %v want %v", item.k, found[item.k], item.v)
		}
	}
	if len(found) != len(all) {
		t.Errorf("iterator count mismatch: got %d values, want %d", len(found), len(all))
	}
}

func TestUnionIterator(t *testing.T) {
	logger := hclog.NewNullLogger()
	dba := NewDatabase(rawdb.NewMemoryDatabase(), logger)
	triea := NewEmpty(dba)
	for _, val := range testdata1 {
		triea.Update([]byte(val.k), []byte(val.v))
	}
	rootA, nodesA, _ := triea.Commit(false)
	dba.Update(NewWithNodeSet(nodesA))
	triea, _ = New(TrieID(rootA), dba, logger)

	dbb := NewDatabase(rawdb.NewMemoryDatabase(), hclog.NewNullLogger())
	trieb := NewEmpty(dbb)
	for _, val := range testdata2 {
		trieb.Update([]byte(val.k), []byte(val.v))
	}
	rootB, nodesB, _ := trieb.Commit(false)
	dbb.Update(NewWithNodeSet(nodesB))
	trieb, _ = New(TrieID(rootB), dbb, logger)

	di, _ := NewUnionIterator([]NodeIterator{triea.NodeIterator(nil), trieb.NodeIterator(nil)})
	it := NewIterator(di)

	all := []struct{ k, v string }{
		{"aardvark", "c"},
		{"barb", "ba"},
		{"barb", "bd"},
		{"bard", "bc"},
		{"bars", "bb"},
		{"bars", "be"},
		{"bar", "b"},
		{"fab", "z"},
		{"food", "ab"},
		{"foos", "aa"},
		{"foo", "a"},
		{"jars", "d"},
	}

	for i, kv := range all {
		if !it.Next() {
			t.Errorf("Iterator ends prematurely at element %d", i)
		}
		if kv.k != string(it.Key) {
			t.Errorf("iterator value mismatch for element %d: got key %s want %s", i, it.Key, kv.k)
		}
		if kv.v != string(it.Value) {
			t.Errorf("iterator value mismatch for element %d: got value %s want %s", i, it.Value, kv.v)
		}
	}
	if it.Next() {
		t.Errorf("Iterator returned extra values.")
	}
}

func TestIteratorNoDups(t *testing.T) {
	tr := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase(), hclog.NewNullLogger()))
	for _, val := range testdata1 {
		tr.Update([]byte(val.k), []byte(val.v))
	}
	checkIteratorNoDups(t, tr.NodeIterator(nil), nil)
}

// This test checks that nodeIterator.Next can be retried after inserting missing trie nodes.
func TestIteratorContinueAfterErrorDisk(t *testing.T)    { testIteratorContinueAfterError(t, false) }
func TestIteratorContinueAfterErrorMemonly(t *testing.T) { testIteratorContinueAfterError(t, true) }

func testIteratorContinueAfterError(t *testing.T, memonly bool) {
	t.Helper()

	diskdb := rawdb.NewMemoryDatabase()
	logger := hclog.NewNullLogger()
	triedb := NewDatabase(diskdb, logger)

	tr := NewEmpty(triedb)
	for _, val := range testdata1 {
		tr.Update([]byte(val.k), []byte(val.v))
	}
	_, nodes, _ := tr.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	if !memonly {
		triedb.Commit(tr.Hash(), true, nil)
	}
	wantNodeCount := checkIteratorNoDups(t, tr.NodeIterator(nil), nil)

	var (
		diskKeys [][]byte
		memKeys  []types.Hash
	)
	if memonly {
		memKeys = triedb.Nodes()
	} else {
		it := diskdb.NewIterator(nil, nil)
		for it.Next() {
			diskKeys = append(diskKeys, it.Key())
		}
		it.Release()
	}
	for i := 0; i < 20; i++ {
		// Create trie that will load all nodes from DB.
		tr, _ := New(TrieID(tr.Hash()), triedb, logger)

		// Remove a random node from the database. It can't be the root node
		// because that one is already loaded.
		var (
			rkey types.Hash
			rval []byte
			robj *cachedNode
		)
		for {
			if memonly {
				rkey = memKeys[rand.Intn(len(memKeys))]
			} else {
				copy(rkey[:], diskKeys[rand.Intn(len(diskKeys))])
			}
			if rkey != tr.Hash() {
				break
			}
		}
		if memonly {
			robj = triedb.dirties[rkey]
			delete(triedb.dirties, rkey)
		} else {
			rval, _, _ = diskdb.Get(rkey[:])
			diskdb.Delete(rkey[:])
		}
		// Iterate until the error is hit.
		seen := make(map[string]bool)
		it := tr.NodeIterator(nil)
		checkIteratorNoDups(t, it, seen)
		missing, ok := it.Error().(*MissingNodeError)
		if !ok || missing.NodeHash != rkey {
			t.Fatal("didn't hit missing node, got", it.Error())
		}

		// Add the node back and continue iteration.
		if memonly {
			triedb.dirties[rkey] = robj
		} else {
			diskdb.Set(rkey[:], rval)
		}
		checkIteratorNoDups(t, it, seen)
		if it.Error() != nil {
			t.Fatal("unexpected error", it.Error())
		}
		if len(seen) != wantNodeCount {
			t.Fatal("wrong node iteration count, got", len(seen), "want", wantNodeCount)
		}
	}
}

// Similar to the test above, this one checks that failure to create nodeIterator at a
// certain key prefix behaves correctly when Next is called. The expectation is that Next
// should retry seeking before returning true for the first time.
func TestIteratorContinueAfterSeekErrorDisk(t *testing.T) {
	testIteratorContinueAfterSeekError(t, false)
}
func TestIteratorContinueAfterSeekErrorMemonly(t *testing.T) {
	testIteratorContinueAfterSeekError(t, true)
}

func testIteratorContinueAfterSeekError(t *testing.T, memonly bool) {
	t.Helper()

	// Commit test trie to db, then remove the node containing "bars".
	diskdb := rawdb.NewMemoryDatabase()
	logger := hclog.NewNullLogger()
	triedb := NewDatabase(diskdb, logger)

	ctr := NewEmpty(triedb)
	for _, val := range testdata1 {
		ctr.Update([]byte(val.k), []byte(val.v))
	}
	root, nodes, _ := ctr.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	if !memonly {
		triedb.Commit(root, true, nil)
	}
	barNodeHash := types.StringToHash("05041990364eb72fcb1127652ce40d8bab765f2bfe53225b1170d276cc101c2e")
	var (
		barNodeBlob []byte
		barNodeObj  *cachedNode
	)
	if memonly {
		barNodeObj = triedb.dirties[barNodeHash]
		delete(triedb.dirties, barNodeHash)
	} else {
		barNodeBlob, _, _ = diskdb.Get(barNodeHash[:])
		diskdb.Delete(barNodeHash[:])
	}
	// Create a new iterator that seeks to "bars". Seeking can't proceed because
	// the node is missing.
	tr, _ := New(TrieID(root), triedb, logger)
	it := tr.NodeIterator([]byte("bars"))
	missing, ok := it.Error().(*MissingNodeError)
	if !ok {
		t.Fatal("want MissingNodeError, got", it.Error())
	} else if missing.NodeHash != barNodeHash {
		t.Fatal("wrong node missing")
	}
	// Reinsert the missing node.
	if memonly {
		triedb.dirties[barNodeHash] = barNodeObj
	} else {
		diskdb.Set(barNodeHash[:], barNodeBlob)
	}
	// Check that iteration produces the right set of values.
	if err := checkIteratorOrder(testdata1[2:], NewIterator(it)); err != nil {
		t.Fatal(err)
	}
}

func checkIteratorNoDups(t *testing.T, it NodeIterator, seen map[string]bool) int {
	t.Helper()

	if seen == nil {
		seen = make(map[string]bool)
	}
	for it.Next(true) {
		if seen[string(it.Path())] {
			t.Fatalf("iterator visited node path %x twice", it.Path())
		}
		seen[string(it.Path())] = true
	}
	return len(seen)
}

type loggingDB struct {
	getCount uint64
	backend  kvdb.KVBatchStorage
}

func (l *loggingDB) Has(key []byte) (bool, error) {
	return l.backend.Has(key)
}

func (l *loggingDB) Get(key []byte) ([]byte, bool, error) {
	l.getCount++

	return l.backend.Get(key)
}

func (l *loggingDB) Set(key []byte, value []byte) error {
	return l.backend.Set(key, value)
}

func (l *loggingDB) Delete(key []byte) error {
	return l.backend.Delete(key)
}

func (l *loggingDB) NewBatch() kvdb.Batch {
	return l.backend.NewBatch()
}

func (l *loggingDB) NewIterator(prefix []byte, start []byte) kvdb.Iterator {
	return l.backend.NewIterator(prefix, start)
}

func (l *loggingDB) Close() error {
	return l.backend.Close()
}

// makeLargeTestTrie create a sample test trie
func makeLargeTestTrie() (*Database, *StateTrie, *loggingDB) {
	// Create an empty trie
	logDB := &loggingDB{0, memorydb.New()}
	logger := hclog.NewNullLogger()
	triedb := NewDatabase(rawdb.NewDatabase(logDB), logger)
	trie, _ := NewStateTrie(TrieID(types.Hash{}), triedb, logger)

	// Fill it with some arbitrary data
	for i := 0; i < 10000; i++ {
		key := make([]byte, 32)
		val := make([]byte, 32)
		binary.BigEndian.PutUint64(key, uint64(i))
		binary.BigEndian.PutUint64(val, uint64(i))
		key = crypto.Keccak256(key)
		val = crypto.Keccak256(val)
		trie.Update(key, val)
	}
	_, nodes, _ := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	// Return the generated trie
	return triedb, trie, logDB
}

// Tests that the node iterator indeed walks over the entire database contents.
func TestNodeIteratorLargeTrie(t *testing.T) {
	// Create some arbitrary test trie to iterate
	db, trie, logDB := makeLargeTestTrie()
	db.Cap(0) // flush everything
	// Do a seek operation
	trie.NodeIterator(types.StringToBytes("0x77667766776677766778855885885885"))
	// master: 24 get operations
	// this pr: 5 get operations
	if have, want := logDB.getCount, uint64(5); have != want {
		t.Fatalf("Too many lookups during seek, have %d want %d", have, want)
	}
}

func TestIteratorNodeBlob(t *testing.T) {
	var (
		db     = rawdb.NewMemoryDatabase()
		triedb = NewDatabase(db, hclog.NewNullLogger())
		trie   = NewEmpty(triedb)
	)
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"dog", "puppy"},
		{"somethingveryoddindeedthis is", "myothernodedata"},
	}
	all := make(map[string]string)
	for _, val := range vals {
		all[val.k] = val.v
		trie.Update([]byte(val.k), []byte(val.v))
	}
	_, nodes, _ := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	triedb.Cap(0)

	found := make(map[types.Hash][]byte)
	it := trie.NodeIterator(nil)
	for it.Next(true) {
		if it.Hash() == (types.Hash{}) {
			continue
		}
		found[it.Hash()] = it.NodeBlob()
	}

	dbIter := db.NewIterator(nil, nil)
	defer dbIter.Release()

	var count int
	for dbIter.Next() {
		got, present := found[types.BytesToHash(dbIter.Key())]
		if !present {
			t.Fatalf("Miss trie node %v", dbIter.Key())
		}
		if !bytes.Equal(got, dbIter.Value()) {
			t.Fatalf("Unexpected trie node want %v got %v", dbIter.Value(), got)
		}
		count += 1
	}
	if count != len(found) {
		t.Fatal("Find extra trie node via iterator")
	}
}