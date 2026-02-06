package chain

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	Endpoint string
	Conn     *websocket.Conn
}

func NewWSClient(endpoint string) *WSClient {
	return &WSClient{Endpoint: endpoint}
}

func (c *WSClient) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, c.Endpoint, nil)
	if err != nil {
		return err
	}
	c.Conn = conn
	return nil
}

func (c *WSClient) Close() {
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
}

func (c *WSClient) Subscribe(ctx context.Context, query string) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "subscribe",
		"params": map[string]any{
			"query": query,
		},
	}
	return c.Conn.WriteJSON(payload)
}

func (c *WSClient) Read(ctx context.Context) ([]byte, error) {
	_, msg, err := c.Conn.ReadMessage()
	return msg, err
}

func ParseWSTx(msg []byte) (*Tx, bool, error) {
	var env struct {
		Result struct {
			Data json.RawMessage `json:"data"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &env); err != nil {
		return nil, false, err
	}
	if env.Error != nil {
		return nil, false, errors.New(env.Error.Message)
	}
	if len(env.Result.Data) == 0 {
		return nil, false, nil
	}

	var data struct {
		Type  string `json:"type"`
		Value struct {
			TxResult struct {
				Height string `json:"height"`
				Hash   string `json:"hash"`
				Tx     string `json:"tx"`
				Result struct {
					Code   int        `json:"code"`
					Events []rpcEvent `json:"events"`
				} `json:"result"`
			} `json:"TxResult"`
		} `json:"value"`
	}
	if err := json.Unmarshal(env.Result.Data, &data); err != nil {
		return nil, false, err
	}
	if !strings.Contains(data.Type, "Tx") {
		return nil, false, nil
	}

	height, err := parseInt64(data.Value.TxResult.Height)
	if err != nil {
		return nil, false, err
	}

	hash := strings.TrimSpace(data.Value.TxResult.Hash)
	if hash == "" && data.Value.TxResult.Tx != "" {
		if h, err := hashFromTx(data.Value.TxResult.Tx); err == nil {
			hash = h
		}
	}

	return &Tx{
		Hash:      strings.ToUpper(hash),
		Height:    height,
		Code:      data.Value.TxResult.Result.Code,
		Events:    decodeEvents(data.Value.TxResult.Result.Events),
		Timestamp: time.Now().UTC(),
	}, true, nil
}

func hashFromTx(txBase64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return strings.ToUpper(hex.EncodeToString(h[:])), nil
}
