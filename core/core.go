package core

import "fmt"

func FormatRedisList(interval int) string {
	return fmt.Sprintf("urls_to_monitor:%v", interval)
}

func FormatRedisHash(interval int) string {
	return fmt.Sprintf("urls_details:%v", interval)
}
