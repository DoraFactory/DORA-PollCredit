package pricing

import "context"

type Service struct {
	FixedCreditPerDora int64
}

type Snapshot struct {
	CreditPerDora int64  `json:"credit_per_dora"`
	Source        string `json:"source"`
}

func (s Service) CurrentSnapshot(ctx context.Context) (Snapshot, error) {
	return Snapshot{
		CreditPerDora: s.FixedCreditPerDora,
		Source:        "fixed",
	}, nil
}
