package fastsync

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/log"
	"github.com/icon-project/goloop/consensus/internal/test"
	"github.com/icon-project/goloop/module"
)

type clientTestSetUp struct {
	*fastSyncTestSetUp
	nms []*test.NetworkManager
	phs []module.ProtocolHandler

	reactors []*tReactor
	m        Manager
	cb       *tFetchCallback
}

func newClientTestSetUp(t *testing.T, n int) *clientTestSetUp {
	s := &clientTestSetUp{}
	s.fastSyncTestSetUp = newFastSyncTestSetUp(t)
	s.nms = make([]*test.NetworkManager, n)
	s.reactors = make([]*tReactor, n)
	s.phs = make([]module.ProtocolHandler, n)
	for i := 0; i < n; i++ {
		s.nms[i] = test.NewNetworkManager()
		if i > 0 {
			s.nms[0].Join(s.nms[i])
			s.reactors[i] = newTReactor()
			var err error
			s.phs[i], err = s.nms[i].RegisterReactorForStreams("fastsync", 0, s.reactors[i], protocols, configFastSyncPriority)
			assert.Nil(t, err)
		}
	}
	var err error
	s.m, err = NewManager(s.nms[0], s.bm, s.bm, log.New())
	assert.Nil(t, err)
	s.cb = newTFetchCallback()
	return s
}

type tOnBlockEvent struct {
	blk module.BlockData
	vs  []byte
	br  BlockResult
}

type tOnEndEvent struct {
	err error
}

type tFetchCallback struct {
	ch chan interface{}
}

func (cb *tFetchCallback) OnBlock(br BlockResult) {
	blk := br.Block()
	vs := br.Votes()
	cb.ch <- tOnBlockEvent{blk, vs, br}
}

func (cb *tFetchCallback) OnEnd(err error) {
	cb.ch <- tOnEndEvent{err}
}

func newTFetchCallback() *tFetchCallback {
	cb := &tFetchCallback{
		ch: make(chan interface{}),
	}
	return cb
}

func (s *clientTestSetUp) assertEqualReceiveEvent(pi module.ProtocolInfo, msg interface{}, id module.PeerID, actual interface{}) {
	b := codec.MustMarshalToBytes(msg)
	assert.Equal(s.t, tReceiveEvent{pi, b, id}, actual)
}

func (s *clientTestSetUp) send(ph module.ProtocolHandler, pi module.ProtocolInfo, msg interface{}, id module.PeerID) {
	bs := codec.MustMarshalToBytes(msg)
	err := ph.Unicast(pi, bs, id)
	assert.Nil(s.t, err)
}

func (s *clientTestSetUp) respondBlockRequest(
	ph module.ProtocolHandler,
	rid uint32,
	blk []byte,
	votes []byte,
	id module.PeerID,
) {
	s.send(ph, ProtoBlockMetadata, &BlockMetadata{rid, int32(len(blk)), votes}, id)
	s.send(ph, ProtoBlockData, &BlockData{rid, blk}, id)
}

func (s *clientTestSetUp) assertBlockEvent(expected []byte, actual interface{}) {
	bev, ok := actual.(tOnBlockEvent)
	assert.True(s.t, ok, "event is not tOnBlockEvent: %s\n", fmt.Sprintf("%T %#v", actual, actual))
	buf := bytes.NewBuffer(nil)
	err := bev.blk.MarshalHeader(buf)
	assert.Nil(s.t, err)
	err = bev.blk.MarshalBody(buf)
	assert.Nil(s.t, err)
	assert.Equal(s.t, expected, buf.Bytes())
}

func (s *clientTestSetUp) assertEndEvent(expected error, actual interface{}) {
	eev, ok := actual.(tOnEndEvent)
	assert.True(s.t, ok, "event is not tOnEndEvent: %s\n", fmt.Sprintf("%T %#v", actual, actual))
	assert.Equal(s.t, expected, eev.err)
}

func (s *clientTestSetUp) assertNoEvent(ch chan interface{}) {
	select {
	case ev := <-ch:
		assert.Failf(s.t, "unexpected event", " %T %#v\n", ev, ev)
	default:
	}
}

func TestClient_Success(t *testing.T) {
	s := newClientTestSetUp(t, 2)
	_, err := s.m.FetchBlocks(1, 10, s.cb)
	assert.Nil(t, err)
	ev := <-s.reactors[1].ch
	s.assertEqualReceiveEvent(ProtoBlockRequest, &BlockRequestV1{0x10000, 1}, s.nms[0].ID, ev)

	s.respondBlockRequest(s.phs[1], 0x10000, s.rawBlocks[1], s.votes[2], s.nms[0].ID)

	ev2 := <-s.cb.ch
	s.assertBlockEvent(s.rawBlocks[1], ev2)
	ev2.(tOnBlockEvent).br.Consume()
}

func TestClient_SuccessMulti(t *testing.T) {
	s := newClientTestSetUp(t, 3)
	_, err := s.m.FetchBlocks(1, 3, s.cb)
	assert.Nil(t, err)

	ev := <-s.reactors[1].ch
	s.assertEqualReceiveEvent(ProtoBlockRequest, &BlockRequestV1{0x10000, 1}, s.nms[0].ID, ev)

	ev = <-s.reactors[2].ch
	s.assertEqualReceiveEvent(ProtoBlockRequest, &BlockRequestV1{0x10000, 2}, s.nms[0].ID, ev)

	s.respondBlockRequest(s.phs[2], 0x10000, s.rawBlocks[2], s.votes[3], s.nms[0].ID)
	s.assertNoEvent(s.cb.ch)

	ev = <-s.reactors[2].ch
	s.assertEqualReceiveEvent(ProtoBlockRequest, &BlockRequestV1{0x10001, 3}, s.nms[0].ID, ev)

	s.respondBlockRequest(s.phs[2], 0x10001, s.rawBlocks[3], s.votes[4], s.nms[0].ID)
	s.assertNoEvent(s.cb.ch)

	s.respondBlockRequest(s.phs[1], 0x10000, s.rawBlocks[1], s.votes[2], s.nms[0].ID)

	ev2 := <-s.cb.ch
	s.assertBlockEvent(s.rawBlocks[1], ev2)
	ev2.(tOnBlockEvent).br.Consume()

	ev2 = <-s.cb.ch
	s.assertBlockEvent(s.rawBlocks[2], ev2)
	ev2.(tOnBlockEvent).br.Consume()

	ev2 = <-s.cb.ch
	s.assertBlockEvent(s.rawBlocks[3], ev2)
	ev2.(tOnBlockEvent).br.Consume()

	ev2 = <-s.cb.ch
	s.assertEndEvent(nil, ev2)
}
