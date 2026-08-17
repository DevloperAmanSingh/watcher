package env

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
)

func LoadEnv(environmentFileName string) {
	err := godotenv.Load(environmentFileName)
	if err != nil {
		log.Fatal("Error loading file:", err)
	}
}

func FetchString(key string, fallback ...string) string {
	response, ok := os.LookupEnv(key)
	if ok {
		return response
	}
	if len(fallback) > 0 {
		return fallback[0]
	}

	// no env var and no fallback: meaning the env MUST BE FOUND or the program will fail
	panic(fmt.Sprintf("environment variable %s is not set and no fallback provided", key))
}

func FetchInt(key string, fallback ...int) int {
	response, ok := os.LookupEnv(key)
	if ok == false && len(fallback) <= 0 {
		panic(fmt.Sprintf("environment variable %s is not set and no fallback provided", key))
	}

	resp, err := strconv.Atoi(response)
	if err != nil {
		if len(fallback) > 0 {
			return fallback[0]
		}
		panic(fmt.Sprintf("environment variable %s is not an integer", key))
	}
	return resp
}
