package database

import (
	"encoding/json"
	"github.com/DevloperAmanSingh/watcher/enums"
	"time"
)

type Url struct {
	Id                  int                       `json:"id"`
	Url                 string                    `json:"url" redis:"url"`
	HttpMethod          enums.HttpMethod          `json:"http_method" redis:"http_method"`
	Status              enums.SiteHealth          `json:"status" redis:"status"`
	MonitoringFrequency enums.MonitoringFrequency `json:"monitoring_frequency" redis:"monitoring_frequency"`
	ContactEmail        string                    `json:"contact_email" redis:"contact_email"`
	CreatedAt           time.Time                 `json:"created_at" redis:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at" redis:"updated_at"`
}

func (url Url) MarshalBinary() (data []byte, err error) {
	bytes, err := json.Marshal(url)
	return bytes, err
}

func (url *Url) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, url)
}
