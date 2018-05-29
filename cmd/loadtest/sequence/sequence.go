// Package sequence implements an in-app sequence provider for the Stellar network,
// independent of orbit nodes.
package sequence

import (
	"strconv"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/laxmicoinofficial/go/build"
	"github.com/laxmicoinofficial/go/clients/orbit"
	"github.com/laxmicoinofficial/go/xdr"
)

// Provider provides sequence numbers for Stellar transactions,
// with local in-app caching. This saves on executing multiple requests to an orbit
// instance for fetching an account's sequence number.
//
// Note this package assumes you are using no more than a single provider for
// an account. Otherwise the returned sequence number will be incorrect
// and out of sync.
type Provider struct {
	build.SequenceProvider
	sync.RWMutex

	client orbit.ClientInterface

	// Local account sequence number cache
	sequences map[string]xdr.SequenceNumber

	logger log.Logger
}

// New receives an orbit client and returns a new Provider instance.
func New(c orbit.ClientInterface, logger log.Logger) *Provider {
	return &Provider{
		RWMutex:   sync.RWMutex{},
		client:    c,
		sequences: make(map[string]xdr.SequenceNumber),
		logger:    logger,
	}
}

// SequenceForAccount returns the sequence number for given account using local cache.
func (p *Provider) SequenceForAccount(address string) (xdr.SequenceNumber, error) {
	// Fetch sequence number from orbit if not found in cache.
	p.RLock()
	seq, ok := p.sequences[address]
	p.RUnlock()

	if !ok {
		var err error
		seq, err = p.LoadSequenceWithClient(address)
		if err != nil {
			return 0, err
		}
	} else {
		level.Debug(p.logger).Log(
			"msg", "sequence number fetched",
			"sequence_number", seq,
			"source_address", address,
			"sequence_provider_source", "local cache")
	}

	return seq, nil
}

// LoadSequenceWithClient loads the sequence number using the provider's orbit.ClientInterface.
// This is in contrast to loading it from local cache.
func (p *Provider) LoadSequenceWithClient(address string) (xdr.SequenceNumber, error) {
	account, err := p.client.LoadAccount(address)
	if err != nil {
		return 0, err
	}

	seqUint, err := strconv.ParseUint(account.Sequence, 10, 64)
	if err != nil {
		return 0, err
	}

	seq := xdr.SequenceNumber(seqUint)

	p.Lock()
	p.sequences[address] = seq
	p.Unlock()

	level.Debug(p.logger).Log(
		"msg", "sequence number fetched",
		"sequence_number", seq,
		"source_address", address,
		"sequence_provider_source", "orbit client")

	return seq, nil
}

// IncrementSequence increments the sequence number for the given account address in the local cache.
func (p *Provider) IncrementSequence(address string) (xdr.SequenceNumber, error) {
	seq, err := p.SequenceForAccount(address)
	if err != nil {
		return 0, err
	}

	newSeq := seq + 1

	p.Lock()
	p.sequences[address] = newSeq
	p.Unlock()

	level.Debug(p.logger).Log("msg", "sequence number incremented", "source_address", address, "sequence_number", newSeq)

	return newSeq, nil
}
