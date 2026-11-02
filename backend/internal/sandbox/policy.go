package sandbox

import (
	"fmt"
	"strings"
)

// ImagePolicy lists the image repositories engines may be started from. The
// backend controls the host's Docker daemon, so only trusted images may run.
//
// A pattern is a repository ("zienek/chess-arena-engine"), a namespace
// ("zienek/*"), or "*" for any image. Tags and digests are ignored.
type ImagePolicy struct {
	patterns []string
}

// NewImagePolicy returns a policy allowing the given patterns.
func NewImagePolicy(patterns []string) ImagePolicy {
	cleaned := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			cleaned = append(cleaned, pattern)
		}
	}
	return ImagePolicy{patterns: cleaned}
}

// Patterns returns the allowed patterns.
func (policy ImagePolicy) Patterns() []string {
	return append([]string(nil), policy.patterns...)
}

// Allows reports whether image matches one of the patterns.
func (policy ImagePolicy) Allows(image string) bool {
	repository := Repository(image)
	for _, pattern := range policy.patterns {
		switch {
		case pattern == "*":
			return true
		case strings.HasSuffix(pattern, "/*"):
			if strings.HasPrefix(repository, strings.TrimSuffix(pattern, "*")) {
				return true
			}
		case repository == pattern:
			return true
		}
	}
	return false
}

// Check returns an error naming the allowed patterns when image is not allowed.
func (policy ImagePolicy) Check(image string) error {
	if policy.Allows(image) {
		return nil
	}
	return fmt.Errorf("image %q is not allowed; allowed images: %s (set ARENA_ALLOWED_IMAGES)",
		image, strings.Join(policy.patterns, ", "))
}

// Repository strips the tag and digest from an image reference.
func Repository(image string) string {
	image = strings.TrimSpace(image)
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	// A colon after the last slash starts the tag; one before it is a registry port.
	if colon := strings.LastIndex(image, ":"); colon > strings.LastIndex(image, "/") {
		image = image[:colon]
	}
	return image
}
