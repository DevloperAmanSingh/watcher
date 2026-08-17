package enums

import (
	"fmt"
	"strings"
)

type DateType string

const (
	Second DateType = "second"
	Minute DateType = "minute"
	Hour   DateType = "hour"
	Day    DateType = "day"
)

func (sh DateType) ToString() string {
	switch sh {
	case Second:
		return "second"
	case Minute:
		return "minute"
	case Hour:
		return "hour"
	case Day:
		return "day"
	default:
		return ""
	}
}

func ParseDataType(s string) (DateType, error) {
	switch strings.ToLower(s) {
	case "second":
		return Second, nil
	case "minute":
		return Minute, nil
	case "hour":
		return Hour, nil
	case "day":
		return Day, nil
	default:
		return "", fmt.Errorf("invalid data type option: %s", s)
	}
}
