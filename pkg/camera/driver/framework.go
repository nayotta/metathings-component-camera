package camera_driver

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type FrameworkOption struct {
	*viper.Viper
}

func (o *FrameworkOption) Sub(key string) *FrameworkOption {
	sub := o.Viper.Sub(key)
	if sub == nil {
		return nil
	}

	return &FrameworkOption{sub}
}

func (o *FrameworkOption) NextKeys() []string {
	m := map[string]bool{}
	for _, k := range o.AllKeys() {
		ss := strings.SplitN(k, ".", 2)
		if len(ss) != 2 {
			continue
		}
		k = ss[0]
		_, ok := m[k]
		if ok {
			continue
		}
		m[k] = true
	}

	a := []string{}
	for x := range m {
		a = append(a, x)
	}

	return a
}

type Framework interface {
	Start() error
	Stop() error
	Wait() <-chan error
}

type FrameworkFactory func(opt *FrameworkOption, args ...interface{}) (Framework, error)

var framework_factories map[string]FrameworkFactory
var framework_factories_once sync.Once

func register_framework_factory(name string, fty FrameworkFactory) {
	framework_factories_once.Do(func() {
		framework_factories = make(map[string]FrameworkFactory)
	})

	framework_factories[name] = fty
}

func NewFramework(name string, opt *FrameworkOption, args ...interface{}) (Framework, error) {
	fty, ok := framework_factories[name]
	if !ok {
		return nil, ErrInvalidFramework
	}

	return fty(opt, args...)
}
