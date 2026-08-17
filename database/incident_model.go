package database

import (
	"encoding/json"
	"time"
)

type Incident struct {
	UrlId      int       `json:"url_id"`
	ResolvedAt time.Time `json:"resolved_at"`
	Time       time.Time `json:"time"`
}

func (incident Incident) MarshalBinary() (data []byte, err error) {
	bytes, err := json.Marshal(incident)
	return bytes, err
}

func (incident *Incident) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, incident)
}
