package service

import (
	"testing"

	"github.com/dedis/cothority/omniledger/collection"
	"github.com/stretchr/testify/require"
)

var (
	coinZero = make([]byte, 8)
	coinOne  = append(make([]byte, 7), byte(1))
	coinTwo  = append(make([]byte, 7), byte(2))
)

func TestContractCoin_Spawn(t *testing.T) {
	// Testing spawning of a new coin and checking it has zero coins in it.
	ct := cvTest{}
	s := Service{}

	inst := Instruction{
		ObjectID: NewObjectID(nil),
		Spawn: &Spawn{
			ContractID: ContractCoinID,
		},
	}

	c := []Coin{}
	sc, co, err := s.ContractCoin(ct, inst, c)
	require.Nil(t, err)
	require.Equal(t, 1, len(sc))
	ca := ObjectID{ZeroDarc, NewNonce(inst.Hash())}
	require.Equal(t, NewStateChange(Create, ca,
		ContractCoinID, coinZero), sc[0])
	require.Equal(t, 0, len(co))
}

func TestContractCoin_InvokeMint(t *testing.T) {
	// Test that a coin can be minted
	ct := newCT()
	s := &Service{}
	coAddr := NewObjectID(nil)
	ct.Store(coAddr, coinZero, ContractCoinID)

	inst := Instruction{
		ObjectID: coAddr,
		Invoke: &Invoke{
			Command: "mint",
			Args:    NewArguments(Argument{"coins", coinOne}),
		},
	}
	sc, co, err := s.ContractCoin(ct, inst, []Coin{})
	require.Nil(t, err)
	require.Equal(t, 0, len(co))
	require.Equal(t, 1, len(sc))
	require.Equal(t, NewStateChange(Update, coAddr, ContractCoinID, coinOne),
		sc[0])
}

func TestContractCoin_InvokeTransfer(t *testing.T) {
	// Test that a coin can be transferred
	ct := newCT()
	s := &Service{}
	coAddr1 := ObjectID{ZeroDarc, ZeroNonce}
	coAddr2 := ObjectID{ZeroDarc, ZeroNonce}
	coAddr2.InstanceID[31] = byte(1)
	ct.Store(coAddr1, coinOne, ContractCoinID)
	ct.Store(coAddr2, coinZero, ContractCoinID)

	// First create an instruction where the transfer should fail
	inst := Instruction{
		ObjectID: coAddr2,
		Invoke: &Invoke{
			Command: "transfer",
			Args: NewArguments(Argument{"coins", coinOne},
				Argument{"destination", coAddr1.Slice()}),
		},
	}
	sc, co, err := s.ContractCoin(ct, inst, []Coin{})
	require.NotNil(t, err)

	inst = Instruction{
		ObjectID: coAddr1,
		Invoke: &Invoke{
			Command: "transfer",
			Args: NewArguments(Argument{"coins", coinOne},
				Argument{"destination", coAddr2.Slice()}),
		},
	}
	sc, co, err = s.ContractCoin(ct, inst, []Coin{})
	require.Nil(t, err)
	require.Equal(t, 0, len(co))
	require.Equal(t, 2, len(sc))
	require.Equal(t, NewStateChange(Update, coAddr2, ContractCoinID, coinOne), sc[0])
	require.Equal(t, NewStateChange(Update, coAddr1, ContractCoinID, coinZero), sc[1])
}

type cvTest struct {
	values      map[string][]byte
	contractIDs map[string]string
}

func newCT() *cvTest {
	return &cvTest{make(map[string][]byte), make(map[string]string)}
}

func (ct cvTest) Get(key []byte) collection.Getter {
	panic("not implemented")
}
func (ct *cvTest) Store(key ObjectID, value []byte, contractID string) {
	k := string(key.Slice())
	ct.values[k] = value
	ct.contractIDs[k] = contractID
}
func (ct cvTest) GetValues(key []byte) (value []byte, contractID string, err error) {
	return ct.values[string(key)], ct.contractIDs[string(key)], nil
}
func (ct cvTest) GetValue(key []byte) ([]byte, error) {
	return ct.values[string(key)], nil
}
func (ct cvTest) GetContractID(key []byte) (string, error) {
	return ct.contractIDs[string(key)], nil
}
