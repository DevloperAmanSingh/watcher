package database

import (
	"encoding/json"
	"time"
)

type UrlStatus struct {
	UrlId  int       `json:"url_id"`
	Status bool      `json:"status"`
	Time   time.Time `json:"time"`
}

func (url UrlStatus) MarshalBinary() (data []byte, err error) {
	bytes, err := json.Marshal(url)
	return bytes, err
}

func (url *UrlStatus) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, url)
}
