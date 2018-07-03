package service

import (
	"fmt"
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

var testSuite = cothority.Suite

func TestCollectTx(t *testing.T) {
	protoPrefix := "TestCollectTx"
	getTx := func(scID skipchain.SkipBlockID) ClientTransactions {
		tx := ClientTransaction{
			Instructions: []Instruction{Instruction{}},
		}
		return ClientTransactions{tx}
	}
	for _, n := range []int{2, 3, 10} {
		protoName := fmt.Sprintf("%s_%d", protoPrefix, n)
		_, err := onet.GlobalProtocolRegister(protoName, NewCollectTxProtocol(getTx))
		require.NoError(t, err)

		local := onet.NewLocalTest(testSuite)
		_, _, tree := local.GenBigTree(n, n, n-1, true)

		p, err := local.CreateProtocol(protoName, tree)
		require.NoError(t, err)

		root := p.(*CollectTxProtocol)
		root.SkipchainID = skipchain.SkipBlockID("hello")
		require.NoError(t, root.Start())

		var txs ClientTransactions
	outer:
		for {
			select {
			case newTxs, more := <-root.TxsChan:
				if more {
					txs = append(txs, newTxs...)
				} else {
					break outer
				}
			}
		}
		require.Equal(t, len(txs), n)
		local.CloseAll()
	}
}
