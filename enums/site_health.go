package enums

import (
	"fmt"
	"strings"
)

type SiteHealth string

const (
	Pending   SiteHealth = "pending"
	Healthy   SiteHealth = "healthy"
	UnHealthy SiteHealth = "unhealthy"
)

func (sh SiteHealth) ToString() string {
	switch sh {
	case Pending:
		return "pending"
	case Healthy:
		return "healthy"
	case UnHealthy:
		return "unhealthy"
	default:
		return ""
	}
}
func ParseSiteHealth(s string) (SiteHealth, error) {
	switch strings.ToLower(s) {
	case "pending":
		return Pending, nil
	case "healthy":
		return Healthy, nil
	case "unhealthy":
		return UnHealthy, nil
	default:
		return "", fmt.Errorf("invalid site health option: %s", s)
	}
}
