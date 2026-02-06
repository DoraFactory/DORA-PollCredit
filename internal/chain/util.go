package chain

import "strings"

func DefaultWSEndpoint(rpc string) string {
	if strings.HasPrefix(rpc, "ws://") || strings.HasPrefix(rpc, "wss://") {
		if strings.HasSuffix(rpc, "/websocket") {
			return rpc
		}
		return strings.TrimRight(rpc, "/") + "/websocket"
	}
	if strings.HasPrefix(rpc, "https://") {
		return "wss://" + strings.TrimPrefix(strings.TrimRight(rpc, "/"), "https://") + "/websocket"
	}
	if strings.HasPrefix(rpc, "http://") {
		return "ws://" + strings.TrimPrefix(strings.TrimRight(rpc, "/"), "http://") + "/websocket"
	}
	return ""
}
