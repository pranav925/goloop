package network

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/icon-project/goloop/module"
)

type Set struct {
	m   map[interface{}]interface{}
	mtx sync.RWMutex
}

func NewSet() *Set {
	return &Set{m: make(map[interface{}]interface{})}
}

func (s *Set) _add(v interface{}, d interface{}) bool {
	if _, ok := s.m[v]; !ok {
		s.m[v] = d
		return true
	}
	return false
}

func (s *Set) _set(v interface{}, d interface{}) interface{} {
	old, ok := s.m[v]
	if ok && old == d {
		return nil
	}
	s.m[v] = d
	return old
}

func (s *Set) _remove(v interface{}) bool {
	if _, ok := s.m[v]; ok {
		delete(s.m, v)
		return true
	}
	return false
}

func (s *Set) _clear() {
	s.m = make(map[interface{}]interface{})
}

func (s *Set) _merge(args ...interface{}) {
	for _, v := range args {
		s._set(v, true)
	}
}

func (s *Set) Add(v interface{}) bool {
	defer s.mtx.Unlock()
	s.mtx.Lock()
	return s._add(v, true)
}

func (s *Set) Set(v interface{}, d interface{}) interface{} {
	defer s.mtx.Unlock()
	s.mtx.Lock()
	return s._set(v, d)
}

func (s *Set) Remove(v interface{}) bool {
	defer s.mtx.Unlock()
	s.mtx.Lock()
	return s._remove(v)
}

func (s *Set) Clear() {
	defer s.mtx.Unlock()
	s.mtx.Lock()
	s._clear()
}

func (s *Set) Merge(args ...interface{}) {
	defer s.mtx.Unlock()
	s.mtx.Lock()
	s._merge(args...)
}

func (s *Set) _contains(v interface{}) bool {
	_, ok := s.m[v]
	return ok
}

func (s *Set) _len() int {
	return len(s.m)
}

func (s *Set) Contains(v interface{}) bool {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	return s._contains(v)
}

func (s *Set) Len() int {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	return s._len()
}

func (s *Set) IsEmpty() bool {
	return s.Len() == 0
}

//Not ordered array
func (s *Set) Array() interface{} {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	arr := make([]interface{}, 0)
	for k := range s.m {
		arr = append(arr, k)
	}
	return arr
}

func (s *Set) Map() map[interface{}]interface{} {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	m := make(map[interface{}]interface{})
	for k, v := range s.m {
		m[k] = v
	}
	return m
}

func (s *Set) String() string {
	m := s.Map()
	return fmt.Sprintf("%v", m)
}

type PeerSet struct {
	ids   *PeerIDSet
	in    *PeerIDSet
	out   *PeerIDSet
	addrs *NetAddressSet
	arr   []*Peer
	rnd   *rand.Rand
	mtx   sync.RWMutex
}

func NewPeerSet() *PeerSet {
	return &PeerSet{
		ids:   NewPeerIDSet(),
		in:    NewPeerIDSet(),
		out:   NewPeerIDSet(),
		addrs: NewNetAddressSet(),
		arr:   make([]*Peer, 0),
		rnd:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *PeerSet) _contains(p *Peer) (r bool) {
	if p.In() {
		r = s.in.Contains(p.ID())
	} else {
		r = s.out.Contains(p.ID())
	}
	return
}

func (s *PeerSet) _shuffle() {
	s.rnd.Shuffle(len(s.arr), func(i, j int) {
		s.arr[i], s.arr[j] = s.arr[j], s.arr[i]
	})
}

type PeerPredicate func(*Peer) bool

func (s *PeerSet) _add(p *Peer, f PeerPredicate) bool {
	defer s.mtx.Unlock()
	s.mtx.Lock()

	if !s._contains(p) && (f == nil || f(p)) {
		if p.In() {
			s.in.Add(p.ID())
		} else {
			s.out.Add(p.ID())
		}
		s.addrs.Add(p.NetAddress())
		s.ids.Add(p.ID())

		s.arr = append(s.arr, p)
		s._shuffle()
		return true
	}
	return false
}

func (s *PeerSet) Add(p *Peer) bool {
	return s._add(p, nil)
}

func (s *PeerSet) AddWithPredicate(p *Peer, f PeerPredicate) bool {
	return s._add(p, f)
}

func (s *PeerSet) Remove(p *Peer) bool {
	defer s.mtx.Unlock()
	s.mtx.Lock()

	if s._contains(p) {
		if p.In() {
			s.in.Remove(p.ID())
			if !s.out.Contains(p.ID()) {
				s.addrs.Remove(p.NetAddress())
				s.ids.Remove(p.ID())
			}
		} else {
			s.out.Remove(p.ID())
			if !s.in.Contains(p.ID()) {
				s.addrs.Remove(p.NetAddress())
				s.ids.Remove(p.ID())
			}
		}

		last := len(s.arr)-1
		for i, tp := range s.arr {
			if tp.In() == p.In() && tp.ID().Equal(p.ID()) {
				s.arr[i] = s.arr[last]
				s.arr = s.arr[:last]
				break
			}
		}
		s._shuffle()
		return true
	}
	return false
}

func (s *PeerSet) Clear() {
	defer s.mtx.Unlock()
	s.mtx.Lock()

	s.in.Clear()
	s.out.Clear()
	s.addrs.Clear()
	s.arr = make([]*Peer, 0)
}

func (s *PeerSet) Contains(p *Peer) bool {
	return s._contains(p)
}

func (s *PeerSet) Merge(args ...*Peer) {
	for _, p := range args {
		s.Add(p)
	}
}

func (s *PeerSet) Array() []*Peer {
	defer s.mtx.RUnlock()
	s.mtx.RLock()

	arr := make([]*Peer, len(s.arr))
	copy(arr, s.arr)
	return arr
}

func (s *PeerSet) GetByID(id module.PeerID) *Peer {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	for _, p := range s.arr {
		if p.ID().Equal(id) {
			return p
		}
	}
	return nil
}

func (s *PeerSet) GetByRole(r PeerRoleFlag, has bool) []*Peer {
	defer s.mtx.RUnlock()
	s.mtx.RLock()

	l := make([]*Peer, 0, len(s.arr))
	for _, p := range s.arr {
		if has == p.HasRole(r) {
			l = append(l, p)
		}
	}
	return l
}

func (s *PeerSet) GetByRecvRole(r PeerRoleFlag, has bool) []*Peer {
	defer s.mtx.RUnlock()
	s.mtx.RLock()

	l := make([]*Peer, 0, len(s.arr))
	for _, p := range s.arr {
		if has == p.HasRecvRole(r) {
			l = append(l, p)
		}
	}
	return l
}

func (s *PeerSet) GetBy(role PeerRoleFlag, has bool, in bool) []*Peer {
	defer s.mtx.RUnlock()
	s.mtx.RLock()

	l := make([]*Peer, 0, len(s.arr))
	for _, p := range s.arr {
		if p.In() == in && has == p.HasRole(role) {
			l = append(l, p)
		}
	}
	return l
}

func (s *PeerSet) NetAddresses() []NetAddress {
	return s.addrs.Array()
}

func (s *PeerSet) HasNetAddress(a NetAddress) bool {
	return s.addrs.Contains(a)
}

func (s *PeerSet) HasNetAddressAndIn(a NetAddress, in bool) bool {
	defer s.mtx.RUnlock()
	s.mtx.RLock()

	for _, p := range s.arr {
		if p.In() == in && p.NetAddress() == a {
			return true
		}
	}
	return false
}

func (s *PeerSet) Find(f func(p *Peer) bool) []*Peer {
	defer s.mtx.RUnlock()
	s.mtx.RLock()

	l := make([]*Peer, 0, len(s.arr))
	for _, p := range s.arr {
		if f(p) {
			l = append(l, p)
		}
	}
	return l
}

func (s *PeerSet) Len() int {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	return len(s.arr)
}

func (s *PeerSet) IsEmpty() bool {
	return s.Len() == 0
}

type NetAddressSet struct {
	*Set
	cache map[NetAddress]string
}

func NewNetAddressSet() *NetAddressSet {
	s := &NetAddressSet{Set: NewSet()}
	s.cache = s.Map()
	return s
}

func (s *NetAddressSet) Add(a NetAddress) bool {
	defer s.Set.mtx.Unlock()
	s.Set.mtx.Lock()

	return s._add(a, "")
}

func (s *NetAddressSet) Data(a NetAddress) (string, bool) {
	defer s.Set.mtx.RUnlock()
	s.Set.mtx.RLock()
	d, ok := s.Set.m[a]
	if ok {
		return d.(string), ok
	}
	return "", ok
}

func (s *NetAddressSet) SetAndRemoveByData(a NetAddress, d string) (old string, removed NetAddress) {
	defer s.Set.mtx.Unlock()
	s.Set.mtx.Lock()
	od := s._set(a, d)
	for k, v := range s.Set.m {
		na := k.(NetAddress)
		if na != a && v == d {
			s._remove(k)
			removed = k.(NetAddress)
		}
	}
	if od != nil {
		old = od.(string)
	}
	return
}

func (s *NetAddressSet) RemoveData(a NetAddress) string {
	defer s.Set.mtx.Unlock()
	s.Set.mtx.Lock()

	old, ok := s.m[a]
	if ok && old != "" {
		s.m[a] = ""
	}
	if old != nil {
		return old.(string)
	}
	return ""
}

func (s *NetAddressSet) Contains(a NetAddress) bool {
	return s.Set.Contains(a)
}

func (s *NetAddressSet) ContainsWithData(a NetAddress, d string) bool {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	v, ok := s.Set.m[a]
	return ok && v == d
}

func (s *NetAddressSet) Clear() {
	defer s.Set.mtx.Unlock()
	s.Set.mtx.Lock()

	s._clear()
	s.cache = s._map()
}

func (s *NetAddressSet) Merge(args ...NetAddress) {
	defer s.Set.mtx.Unlock()
	s.Set.mtx.Lock()

	//Add
	for _, a := range args {
		if _, ok := s.Set.m[a]; !ok {
			s.Set.m[a] = ""
		}
		if _, ok := s.cache[a]; ok {
			delete(s.cache, a)
		}
	}

	//Remove
	for k := range s.cache {
		if d := s.Set.m[k]; d == "" {
			delete(s.Set.m, k)
		}
	}
	s.cache = s._map()
}

func (s *NetAddressSet) Array() []NetAddress {
	defer s.Set.mtx.RUnlock()
	s.Set.mtx.RLock()
	arr := make([]NetAddress, 0)
	for k := range s.Set.m {
		arr = append(arr, k.(NetAddress))
	}
	return arr
}

func (s *NetAddressSet) ClearAndAdd(args ...NetAddress) {
	s.Clear()
	s.Merge(args...)
}

func (s *NetAddressSet) _map() map[NetAddress]string {
	m := make(map[NetAddress]string)
	for k, v := range s.Set.m {
		m[k.(NetAddress)] = v.(string)
	}
	return m
}

func (s *NetAddressSet) Map() map[NetAddress]string {
	defer s.Set.mtx.RUnlock()
	s.Set.mtx.RLock()
	return s._map()
}

type PeerIDSet struct {
	*Set
	version  int64
	onUpdate func(*PeerIDSet)
}

func NewPeerIDSet() *PeerIDSet {
	s := &PeerIDSet{Set: NewSet(), onUpdate: func(*PeerIDSet) {}}
	return s
}

func NewPeerIDSetFromBytes(b []byte) (*PeerIDSet, []byte) {
	s := NewPeerIDSet()
	tb := b[:]
	l := len(b)
	for peerIDSize <= l {
		id := NewPeerID(tb[:peerIDSize])
		tb = tb[peerIDSize:]
		s.Add(id)
		l -= peerIDSize
	}
	return s, tb[:]
}

func (s *PeerIDSet) _update() {
	if s.onUpdate != nil {
		s.onUpdate(s)
	}
}

func (s *PeerIDSet) _contains(v interface{}) (bool, module.PeerID) {
	for k := range s.Set.m {
		if k.(module.PeerID).Equal(v.(module.PeerID)) {
			return true, k.(module.PeerID)
		}
	}
	return false, nil
}

func (s *PeerIDSet) Add(id module.PeerID) (r bool) {
	defer func() {
		s.Set.mtx.Unlock()
		if r {
			s._update()
		}
	}()
	s.Set.mtx.Lock()
	if ok, _ := s._contains(id); !ok {
		s.Set.m[id] = 1
		r = true
	}
	return
}

func (s *PeerIDSet) Remove(id module.PeerID) (r bool) {
	defer func() {
		s.Set.mtx.Unlock()
		if r {
			s._update()
		}
	}()
	s.Set.mtx.Lock()

	if ok, k := s._contains(id); ok {
		delete(s.Set.m, k)
		r = true
	}
	return
}

func (s *PeerIDSet) Removes(args ...module.PeerID) {
	var r bool
	defer func() {
		s.Set.mtx.Unlock()
		if r {
			s._update()
		}
	}()
	s.Set.mtx.Lock()
	for _, id := range args {
		if ok, k := s._contains(id); ok {
			delete(s.Set.m, k)
			r = true
		}
	}
}

func (s *PeerIDSet) Contains(id module.PeerID) bool {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	ok, _ := s._contains(id)
	return ok
}

func (s *PeerIDSet) Merge(args ...module.PeerID) {
	var r bool
	defer func() {
		s.Set.mtx.Unlock()
		if r {
			s._update()
		}
	}()
	s.Set.mtx.Lock()
	for _, id := range args {
		if ok, _ := s._contains(id); !ok {
			s.Set.m[id] = 1
			r = true
		}
	}
}

func (s *PeerIDSet) Array() []module.PeerID {
	defer s.Set.mtx.RUnlock()
	s.Set.mtx.RLock()
	arr := make([]module.PeerID, 0)
	for k := range s.Set.m {
		arr = append(arr, k.(module.PeerID))
	}
	return arr
}

func (s *PeerIDSet) ClearAndAdd(args ...module.PeerID) {
	s.Clear()
	s.Merge(args...)
}

func (s *PeerIDSet) Bytes() []byte {
	defer s.Set.mtx.RUnlock()
	s.Set.mtx.RLock()

	arr := make([]byte, s._len()*peerIDSize)
	b := arr[:]
	for k := range s.Set.m {
		id := k.(module.PeerID)
		copy(b, id.Bytes())
		b = b[peerIDSize:]
	}
	return arr[:]
}

type _bytes struct {
	b []byte
}

type BytesSet struct {
	*Set
	size int
}

func NewBytesSet(size int) *BytesSet {
	s := &BytesSet{Set: NewSet(), size: size}
	return s
}

func NewBytesSetFromBytes(b []byte, size int) (*BytesSet, []byte) {
	s := NewBytesSet(size)
	tb := b[:]
	l := len(b)
	for size <= l {
		s.Add(tb[:size])
		tb = tb[size:]
		l -= size
	}
	return s, tb[:]
}

func (s *BytesSet) _contains(b []byte) bool {
	for k := range s.Set.m {
		tb := k.(*_bytes)
		if bytes.Equal(tb.b, b) {
			return true
		}
	}
	return false
}

func (s *BytesSet) _get(b []byte) *_bytes {
	for k := range s.Set.m {
		tb := k.(*_bytes)
		if bytes.Equal(tb.b, b) {
			return tb
		}
	}
	return nil
}

func (s *BytesSet) Add(b []byte) (r bool) {
	defer func() {
		s.Set.mtx.Unlock()
	}()
	s.Set.mtx.Lock()
	if !s._contains(b) {
		tb := &_bytes{b: make([]byte, s.size)}
		copy(tb.b, b)
		s.Set.m[tb] = 1
		r = true
	}
	return
}

func (s *BytesSet) Remove(b []byte) (r bool) {
	defer func() {
		s.Set.mtx.Unlock()
	}()
	s.Set.mtx.Lock()
	if tb := s._get(b); tb != nil {
		delete(s.Set.m, tb)
		r = true
	}
	return
}

func (s *BytesSet) Contains(b []byte) bool {
	defer s.mtx.RUnlock()
	s.mtx.RLock()
	return s._contains(b)
}

func (s *BytesSet) Bytes() []byte {
	defer s.Set.mtx.RUnlock()
	s.Set.mtx.RLock()

	arr := make([]byte, s._len()*s.size)
	tb := arr[:]
	for k := range s.Set.m {
		b := k.(*_bytes)
		copy(tb, b.b)
		tb = tb[s.size:]
	}
	return arr[:]
}

type RoleSet struct {
	*Set
}

func NewRoleSet() *RoleSet {
	return &RoleSet{Set: NewSet()}
}
