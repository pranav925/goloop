package service

import (
	"math/big"
	"time"

	"github.com/icon-project/goloop/common/errors"
	"github.com/icon-project/goloop/module"
	"github.com/icon-project/goloop/service/contract"
	"github.com/icon-project/goloop/service/state"
	"github.com/icon-project/goloop/service/transaction"
	"github.com/icon-project/goloop/service/txresult"
)

func (t *transition) executeTxsSequential(l module.TransactionList, ctx contract.Context, rctBuf []txresult.Receipt) error {
	skipping := ctx.SkipTransactionEnabled()
	cnt := 0
	for i := l.Iterator(); i.Has(); i.Next() {
		if t.step == stepCanceled {
			return ErrTransitionInterrupted
		}
		txi, _, err := i.Get()
		if err != nil {
			t.log.Errorf("Fail to iterate transaction list err=%+v", err)
			return err
		}
		txo := txi.(transaction.Transaction)
		if skipping && txo.IsSkippable() {
			t.log.Tracef("SKIP TX <0x%x>", txo.ID())
			zero := big.NewInt(0)
			rct := txresult.NewReceipt(t.db, ctx.Revision(), txo.To())
			rct.SetResult(module.StatusSkipTransaction, zero, zero, nil)
			rctBuf[cnt] = rct
			cnt++
			continue
		}
		t.log.Tracef("START TX <0x%x>", txo.ID())
		ts := time.Now()
		for trial := 0; ; trial++ {
			txh, err := txo.GetHandler(t.cm)
			if err != nil {
				t.log.Errorf("Fail to GetHandler err=%+v", err)
				return err
			}
			ctx.SetTransactionInfo(&state.TransactionInfo{
				Group:     txo.Group(),
				Index:     int32(cnt),
				Timestamp: txo.Timestamp(),
				Nonce:     txo.Nonce(),
				Hash:      txo.ID(),
				From:      txo.From(),
			})
			ctx.UpdateSystemInfo()
			rct, err := txh.Execute(ctx, false)
			txh.Dispose()
			if err == nil {
				err = t.plt.OnTransactionEnd(ctx, t.log, rct)
			}
			if err == nil {
				rctBuf[cnt] = rct
				break
			}
			if !errors.ExecutionFailError.Equals(err) {
				t.log.Warnf("Fail to execute transaction err=%+v", err)
				return err
			}
			if trial == RetryCount {
				t.log.Warnf("Fail to execute transaction retry=%d err=%+v", trial, err)
				return err
			}
			t.log.Warnf("RETRY TX <%#x> for err=%+v", txo.ID(), err)
			ts = time.Now()
		}
		duration := time.Now().Sub(ts)
		t.log.Tracef("END   TX <0x%x> duration=%s", txo.ID(), duration)
		cnt++
	}
	return nil
}
