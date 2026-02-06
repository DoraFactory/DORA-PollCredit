package chain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RPCClient struct {
	baseURL string
	client  *http.Client
}

func NewRPCClient(baseURL string) *RPCClient {
	return &RPCClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *RPCClient) LatestHeight(ctx context.Context) (int64, error) {
	endpoint := c.baseURL + "/status"
	var resp statusResponse
	if err := c.getJSON(ctx, endpoint, &resp); err != nil {
		return 0, err
	}
	return parseInt64(resp.Result.SyncInfo.LatestBlockHeight)
}

func (c *RPCClient) TxSearch(ctx context.Context, query string, page, perPage int) (*TxSearchResult, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 30
	}
	u, err := url.Parse(c.baseURL + "/tx_search")
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	// Tendermint/CometBFT docs use query="...".
	values.Set("query", "\""+query+"\"")
	values.Set("prove", "false")
	values.Set("page", strconv.Itoa(page))
	values.Set("per_page", strconv.Itoa(perPage))
	values.Set("order_by", "\"asc\"")
	u.RawQuery = values.Encode()
	endpoint := u.String()
	var resp txSearchResponse
	if err := c.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	result := &TxSearchResult{}
	total, err := parseInt64(resp.Result.TotalCount)
	if err != nil {
		return nil, err
	}
	result.TotalCount = total

	for _, tx := range resp.Result.Txs {
		height, err := parseInt64(tx.Height)
		if err != nil {
			return nil, err
		}
		timestamp, _ := time.Parse(time.RFC3339, tx.Timestamp)
		result.Txs = append(result.Txs, Tx{
			Hash:      tx.Hash,
			Height:    height,
			Code:      tx.TxResult.Code,
			Events:    decodeEvents(tx.TxResult.Events),
			Timestamp: timestamp,
		})
	}
	return result, nil
}

func (c *RPCClient) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("rpc http status %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("rpc http status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func parseInt64(v string) (int64, error) {
	if v == "" {
		return 0, errors.New("empty int string")
	}
	return strconv.ParseInt(v, 10, 64)
}

func decodeEvents(events []rpcEvent) []Event {
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		e := Event{Type: ev.Type}
		for _, attr := range ev.Attributes {
			e.Attributes = append(e.Attributes, Attribute{
				Key:   decodeMaybeBase64(attr.Key),
				Value: decodeMaybeBase64(attr.Value),
			})
		}
		out = append(out, e)
	}
	return out
}

func decodeMaybeBase64(v string) string {
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return v
	}
	if isMostlyPrintable(b) {
		return string(b)
	}
	return v
}

func isMostlyPrintable(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	printable := 0
	for _, c := range b {
		if c >= 32 && c <= 126 {
			printable++
		}
	}
	return printable*100/len(b) >= 80
}

// RPC response types

type statusResponse struct {
	Result struct {
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
		} `json:"sync_info"`
	} `json:"result"`
}

type txSearchResponse struct {
	Result struct {
		TotalCount string  `json:"total_count"`
		Txs        []rpcTx `json:"txs"`
	} `json:"result"`
}

type rpcTx struct {
	Hash      string     `json:"hash"`
	Height    string     `json:"height"`
	Timestamp string     `json:"timestamp"`
	TxResult  rpcTxResult `json:"tx_result"`
}

type rpcTxResult struct {
	Code   int        `json:"code"`
	Events []rpcEvent `json:"events"`
}

type rpcEvent struct {
	Type       string        `json:"type"`
	Attributes []rpcAttribute `json:"attributes"`
}

type rpcAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Parsed types

type TxSearchResult struct {
	TotalCount int64
	Txs        []Tx
}

type Tx struct {
	Hash      string
	Height    int64
	Code      int
	Events    []Event
	Timestamp time.Time
}

type Event struct {
	Type       string
	Attributes []Attribute
}

type Attribute struct {
	Key   string
	Value string
}
