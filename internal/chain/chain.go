package chain

import "context"

type Client struct {
	RPCEndpoint string
}

func NewClient(rpc string) *Client {
	return &Client{RPCEndpoint: rpc}
}

// TODO: implement RPC/WS listeners and tx_search backfill.
func (c *Client) Health(ctx context.Context) error {
	return nil
}
