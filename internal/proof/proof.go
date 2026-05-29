package proof

type Status string

const (
	StatusProved         Status = "proved"
	StatusChecked        Status = "checked"
	StatusCounterexample Status = "counterexample"
	StatusAssumed        Status = "assumed"
	StatusNotSupported   Status = "not_supported"
)

type Obligation struct {
	Ref       string `json:"ref"`
	Status    Status `json:"status"`
	Formula   string `json:"formula"`
	Reason    string `json:"reason"`
	Producer  string `json:"producer"`
	Evidence  string `json:"evidence,omitempty"`
	Bound     int    `json:"bound,omitempty"`
	Witness   string `json:"witness,omitempty"`
	Discharge string `json:"suggested_discharge,omitempty"`
}

type Hole struct {
	Ref       string `json:"ref"`
	Status    Status `json:"status"`
	Reason    string `json:"reason"`
	Discharge string `json:"suggested_discharge"`
}

func Holes(obligations []Obligation) []Hole {
	var holes []Hole
	for _, obligation := range obligations {
		switch obligation.Status {
		case StatusAssumed, StatusNotSupported:
			holes = append(holes, Hole{
				Ref:       obligation.Ref,
				Status:    obligation.Status,
				Reason:    obligation.Reason,
				Discharge: obligation.Discharge,
			})
		}
	}
	return holes
}
