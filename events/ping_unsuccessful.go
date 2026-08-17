package events

type PingUnSuccessful struct {
	UrlId   int
	Healthy bool
	Url     string
}

func (p *PingUnSuccessful) Name() string {
	return "ping.unsuccessful"
}
