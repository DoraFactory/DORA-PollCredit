package worker

import "DORAPollCredit/internal/chain"

type transfer struct {
	Recipient string
	Amount    string
	Sender    string
}

func extractTransfers(events []chain.Event, denom string) []transfer {
	var out []transfer
	for _, ev := range events {
		switch ev.Type {
		case "transfer":
			var amt string
			var rec string
			var snd string
			for _, attr := range ev.Attributes {
				switch attr.Key {
				case "amount":
					amt = attr.Value
				case "recipient":
					rec = attr.Value
				case "sender":
					snd = attr.Value
				}
			}
			if rec != "" {
				if parsed, ok := parseAmountForDenom(amt, denom); ok {
					out = append(out, transfer{Recipient: rec, Amount: parsed, Sender: snd})
				}
			}
		case "coin_received":
			var amt string
			var rec string
			for _, attr := range ev.Attributes {
				switch attr.Key {
				case "amount":
					amt = attr.Value
				case "receiver":
					rec = attr.Value
				}
			}
			if rec != "" {
				if parsed, ok := parseAmountForDenom(amt, denom); ok {
					out = append(out, transfer{Recipient: rec, Amount: parsed})
				}
			}
		}
	}
	return out
}
