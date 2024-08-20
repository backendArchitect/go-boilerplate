package utils

import "github.com/google/uuid"

func Contains(item string, items []string) bool {
	for _, i := range items {
		if i == item {
			return true
		}
	}
	return false
}

func GenerateUUID() string {
	id := uuid.New()
	return id.String()
}
