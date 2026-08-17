package events

type PingSuccessful struct {
	UrlId   int
	Healthy bool
	Url     string
}

func (p *PingSuccessful) Name() string {
	return "ping.successful"
}
