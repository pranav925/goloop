package service

import (
	"github.com/icon-project/goloop/common"
	"github.com/icon-project/goloop/module"
	"math/big"
)

type worldContext struct {
	WorldState

	timeStamp   int64
	blockHeight int64
}

func (c *worldContext) WorldVirtualState() WorldVirtualState {
	if wvs, ok := c.WorldState.(WorldVirtualState); ok {
		return wvs
	}
	return NewWorldVirtualState(c.WorldState, nil)
}

func (c *worldContext) GetFuture(lq []LockRequest) WorldContext {
	wvs := c.WorldVirtualState()
	if len(lq) == 0 {
		return c.WorldStateChanged(wvs)
	} else {
		return c.WorldStateChanged(wvs.GetFuture(lq))
	}
}

var stepPrice = big.NewInt(10 * GIGA)

func (c *worldContext) StepPrice() *big.Int {
	// TODO We need to access world state to get valid value.
	return stepPrice
}

func (c *worldContext) TimeStamp() int64 {
	return c.timeStamp
}

func (c *worldContext) BlockHeight() int64 {
	return c.blockHeight
}

var treasury = common.NewAddressFromString("hx1000000000000000000000000000000000000000")

func (c *worldContext) Treasury() module.Address {
	return treasury
}

func (c *worldContext) WorldStateChanged(ws WorldState) WorldContext {
	return &worldContext{
		WorldState:  ws,
		timeStamp:   c.timeStamp,
		blockHeight: c.blockHeight,
	}
}

func NewWorldContext(ws WorldState, ts int64, height int64) WorldContext {
	return &worldContext{
		WorldState:  ws,
		timeStamp:   ts,
		blockHeight: height,
	}
}
