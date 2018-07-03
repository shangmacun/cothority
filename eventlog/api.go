package eventlog

import (
	"bytes"
	"errors"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/darc/expression"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/protobuf"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
)

// Client is a structure to communicate with the eventlog service
type Client struct {
	OmniLedger *omniledger.Client
	// Signers are the Darc signers that will sign transactions sent with this client.
	Signers    []darc.Signer
	EventlogID omniledger.InstanceID
	c          *onet.Client
}

// NewClient creates a new client to talk to the eventlog service.
func NewClient(ol *omniledger.Client) *Client {
	return &Client{
		OmniLedger: ol,
		c:          onet.NewClient(cothority.Suite, ServiceName),
	}
}

// AddWriter modifies the given darc.Rules to use expr as the authorized writer
// to add new Event Logs. If expr is nil, the current evolution expression is
// used instead.
func AddWriter(r darc.Rules, expr expression.Expr) darc.Rules {
	if expr == nil {
		expr = r.GetEvolutionExpr()
	}
	r["spawn:eventlog"] = expr
	r["invoke:eventlog"] = expr
	return r
}

// Create creates a new event log. Upon return, c.EventlogID will be correctly
// set. This method is synchronous: it will only return once the new eventlog has
// been committed into the OmniLedger (or after a timeout).
func (c *Client) Create(d darc.ID) error {
	instr := omniledger.Instruction{
		InstanceID: omniledger.InstanceID{
			DarcID: d,
		},
		Index:  0,
		Length: 1,
		Spawn:  &omniledger.Spawn{ContractID: contractName},
	}
	if err := instr.SignBy(c.Signers...); err != nil {
		return err
	}
	tx := omniledger.ClientTransaction{
		Instructions: []omniledger.Instruction{instr},
	}
	if _, err := c.OmniLedger.AddTransaction(tx); err != nil {
		return err
	}

	var subID omniledger.SubID
	copy(subID[:], instr.Hash())
	c.EventlogID = omniledger.InstanceID{
		DarcID: d,
		SubID:  subID,
	}

	// Wait for GetProof to see it or timeout.
	cfg, err := c.OmniLedger.GetChainConfig()
	if err != nil {
		return err
	}

	found := false
	for ct := 0; ct < 10; ct++ {
		resp, err := c.OmniLedger.GetProof(c.EventlogID.Slice())
		if err == nil {
			if resp.Proof.InclusionProof.Match() {
				found = true
				break
			}
		}
		time.Sleep(cfg.BlockInterval)
	}
	if !found {
		return errors.New("timeout waiting for creation")
	}

	return nil
}

// A LogID is an opaque unique identifier useful to find a given log message later
// via omniledger.GetProof.
type LogID []byte

// Log asks the service to log events.
func (c *Client) Log(ev ...Event) ([]LogID, error) {
	tx, keys, err := makeTx(c.EventlogID, ev, c.Signers)
	if err != nil {
		return nil, err
	}
	if _, err := c.OmniLedger.AddTransaction(*tx); err != nil {
		return nil, err
	}
	return keys, nil
}

// GetEvent asks the service to retrieve an event.
func (c *Client) GetEvent(key []byte) (*Event, error) {
	reply, err := c.OmniLedger.GetProof(key)
	if err != nil {
		return nil, err
	}
	if !reply.Proof.InclusionProof.Match() {
		return nil, errors.New("not an inclusion proof")
	}
	k, vs, err := reply.Proof.KeyValue()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(k, key) {
		return nil, errors.New("wrong key")
	}
	if len(vs) < 2 {
		return nil, errors.New("not enough values")
	}
	e := Event{}
	err = protobuf.Decode(vs[0], &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func makeTx(eventlogID omniledger.InstanceID, msgs []Event, signers []darc.Signer) (*omniledger.ClientTransaction, []LogID, error) {
	// We need the identity part of the signatures before
	// calling ToDarcRequest() below, because the identities
	// go into the message digest.
	sigs := make([]darc.Signature, len(signers))
	for i, x := range signers {
		sigs[i].Signer = x.Identity()
	}

	keys := make([]LogID, len(msgs))

	instrNonce := omniledger.GenNonce()
	tx := omniledger.ClientTransaction{
		Instructions: make([]omniledger.Instruction, len(msgs)),
	}
	for i, msg := range msgs {
		eventBuf, err := protobuf.Encode(&msg)
		if err != nil {
			return nil, nil, err
		}
		argEvent := omniledger.Argument{
			Name:  "event",
			Value: eventBuf,
		}
		tx.Instructions[i] = omniledger.Instruction{
			InstanceID: eventlogID,
			Nonce:      instrNonce,
			Index:      i,
			Length:     len(msgs),
			Invoke: &omniledger.Invoke{
				Command: contractName,
				Args:    []omniledger.Argument{argEvent},
			},
			Signatures: append([]darc.Signature{}, sigs...),
		}
	}
	for i := range tx.Instructions {
		darcSigs := make([]darc.Signature, len(signers))
		for j, signer := range signers {
			dr, err := tx.Instructions[i].ToDarcRequest()
			if err != nil {
				return nil, nil, err
			}

			sig, err := signer.Sign(dr.Hash())
			if err != nil {
				return nil, nil, err
			}
			darcSigs[j] = darc.Signature{
				Signature: sig,
				Signer:    signer.Identity(),
			}
		}
		tx.Instructions[i].Signatures = darcSigs
		keys[i] = LogID(tx.Instructions[i].DeriveID("event").Slice())
	}
	return &tx, keys, nil
}

// Search executes a search on the filter in req. See the definition of
// type SearchRequest for additional details about how the filter is interpreted.
// The ID field of the SearchRequest will be filled in from c, if it is null.
func (c *Client) Search(req *SearchRequest) (*SearchResponse, error) {
	if req.ID.IsNull() {
		req.ID = c.OmniLedger.ID
	}
	reply := &SearchResponse{}
	if err := c.c.SendProtobuf(c.OmniLedger.Roster.List[0], req, reply); err != nil {
		return nil, err
	}
	return reply, nil
}
