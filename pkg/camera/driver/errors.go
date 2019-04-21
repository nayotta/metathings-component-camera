package camera_driver

import "errors"

var (
	ErrInvalidCameraDriver = errors.New("invalid camera driver")
	ErrInvalidConfig       = errors.New("invalid config")
	ErrNotStartable        = errors.New("not startable")
	ErrNotStoppable        = errors.New("not stoppable")
)
