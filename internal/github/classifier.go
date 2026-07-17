package github

import (
	"fmt"
	"slices"
)

var classifierValues = []struct {
	flag string
	enum string
}{
	{"abuse", "ABUSE"},
	{"duplicate", "DUPLICATE"},
	{"low-quality", "LOW_QUALITY"},
	{"off-topic", "OFF_TOPIC"},
	{"outdated", "OUTDATED"},
	{"resolved", "RESOLVED"},
	{"spam", "SPAM"},
}

func AllowedReasons() []string {
	reasons := make([]string, 0, len(classifierValues))
	for _, value := range classifierValues {
		reasons = append(reasons, value.flag)
	}
	return reasons
}

func ParseReason(value string) (string, error) {
	for _, classifier := range classifierValues {
		if classifier.flag == value {
			return classifier.enum, nil
		}
	}

	return "", fmt.Errorf("invalid reason %q (must be one of: %v)", value, AllowedReasons())
}

func IsAllowedReason(value string) bool {
	return slices.Contains(AllowedReasons(), value)
}
