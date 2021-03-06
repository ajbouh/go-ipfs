package reprovide

import (
	"context"
	"fmt"
	"time"

	backoff "github.com/cenkalti/backoff"
	blocks "github.com/ipfs/go-ipfs/blocks/blockstore"
	logging "github.com/ipfs/go-log"
	routing "github.com/libp2p/go-libp2p-routing"
)

var log = logging.Logger("reprovider")

type Reprovider struct {
	// The routing system to provide values through
	rsys routing.ContentRouting

	// The backing store for blocks to be provided
	bstore blocks.Blockstore
}

func NewReprovider(rsys routing.ContentRouting, bstore blocks.Blockstore) *Reprovider {
	return &Reprovider{
		rsys:   rsys,
		bstore: bstore,
	}
}

func (rp *Reprovider) ProvideEvery(ctx context.Context, tick time.Duration) {
	// dont reprovide immediately.
	// may have just started the daemon and shutting it down immediately.
	// probability( up another minute | uptime ) increases with uptime.
	after := time.After(time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-after:
			err := rp.Reprovide(ctx)
			if err != nil {
				log.Debug(err)
			}
			after = time.After(tick)
		}
	}
}

func (rp *Reprovider) Reprovide(ctx context.Context) error {
	keychan, err := rp.bstore.AllKeysChan(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get key chan from blockstore: %s", err)
	}
	for c := range keychan {
		op := func() error {
			err := rp.rsys.Provide(ctx, c, true)
			if err != nil {
				log.Debugf("Failed to provide key: %s", err)
			}
			return err
		}

		// TODO: this backoff library does not respect our context, we should
		// eventually work contexts into it. low priority.
		err := backoff.Retry(op, backoff.NewExponentialBackOff())
		if err != nil {
			log.Debugf("Providing failed after number of retries: %s", err)
			return err
		}
	}
	return nil
}
