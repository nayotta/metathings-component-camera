package camera_driver

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidCameraDriver = errors.New("invalid camera driver")
	ErrInvalidFramework    = errors.New("invalid framework")
	ErrNotStartable        = errors.New("not startable")
	ErrNotStoppable        = errors.New("not stoppable")
)

func new_invalid_config_error(key string) error {
	return errors.New(fmt.Sprintf("invalid config: %s", key))
}
