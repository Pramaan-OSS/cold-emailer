package models

type DiagnosticParticipation struct {
	Mode                  TestParticipationMode `json:"mode"`
	Send                  bool                  `json:"send"`
	Receive               bool                  `json:"receive"`
	SharedDailyLimit      *int                  `json:"shared_daily_limit,omitempty"`
	RollingRecipientLimit *int                  `json:"rolling_recipient_limit,omitempty"`
}

func (p DiagnosticParticipation) Valid() bool {
	return (p.Mode == TestParticipationDiagnostic || p.Mode == TestParticipationOff && !p.Send && !p.Receive) &&
		(p.SharedDailyLimit == nil || *p.SharedDailyLimit >= 0) && (p.RollingRecipientLimit == nil || *p.RollingRecipientLimit >= 0)
}
