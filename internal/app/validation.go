package app

import (
	"errors"
	"fmt"
	"regexp"
)

const ControlPlaneName = "zeroku"

var namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func ValidateRouteName(name string) error {
	if name == "" {
		return errors.New("app name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("app name must be 63 characters or fewer, got %d", len(name))
	}
	if !namePattern.MatchString(name) {
		return errors.New("app name may contain lowercase letters, digits, and hyphens, but must not start or end with a hyphen")
	}
	if name == ControlPlaneName {
		return fmt.Errorf("%q is reserved", name)
	}
	return nil
}
