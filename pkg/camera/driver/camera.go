package camera_driver

import (
	"sync"

	"github.com/nayotta/viper"
)

type CameraDriverOption struct {
	*viper.Viper
}

func (o *CameraDriverOption) Sub(key string) *CameraDriverOption {
	sub := o.Viper.Sub(key)
	if sub == nil {
		return nil
	}

	return &CameraDriverOption{sub}
}

type CameraDriverState struct {
	state string
}

func (s *CameraDriverState) String() string {
	return s.state
}

var (
	CAMERA_DRIVER_STATE_ON  = &CameraDriverState{state: "on"}
	CAMERA_DRIVER_STATE_OFF = &CameraDriverState{state: "off"}
)

type CameraDriver interface {
	Start() error
	Stop() error
	State() *CameraDriverState
}

type CameraDriverFactory func(opt *CameraDriverOption, args ...interface{}) (CameraDriver, error)

var camera_driver_factories map[string]CameraDriverFactory
var camera_driver_factories_once sync.Once

func register_camera_driver_factory(name string, fty CameraDriverFactory) {
	camera_driver_factories_once.Do(func() {
		camera_driver_factories = make(map[string]CameraDriverFactory)
	})

	camera_driver_factories[name] = fty
}

func NewCameraDriver(name string, opt *CameraDriverOption, args ...interface{}) (CameraDriver, error) {
	fty, ok := camera_driver_factories[name]
	if !ok {
		return nil, ErrInvalidCameraDriver
	}

	return fty(opt, args...)
}
