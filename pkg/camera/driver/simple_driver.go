package camera_driver

import (
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"path"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	opt_helper "github.com/nayotta/metathings/pkg/common/option"
	component "github.com/nayotta/metathings/pkg/component"
)

/*
 * Driver: simple
 *   for livego rtmp server, generate random live id.
 * Options:
 *   driver:
 *     name: simple
 *     inputs:
 *       0:
 *         file: <path>  // file path, like `/dev/video0` etc.
 *     outputs:
 *       0:
 *         file_prefix: <path>  // file path prefix, like `rtmp://rtmp-server:1935/path`.
 *     framework:
 *        ...
 */

type SimpleCameraDriver struct {
	op_mtx *sync.Mutex
	frmwrk Framework

	logger log.FieldLogger
	mdl    *component.Module
	opt    *CameraDriverOption
	st     *CameraDriverState
}

const _LIVEID_LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func random_strings(n int) string {
	buf := make([]byte, n)
	for i := 0; i < n; i++ {
		buf[i] = _LIVEID_LETTERS[rand.Intn(len(_LIVEID_LETTERS))]
	}
	return string(buf)
}

func (d *SimpleCameraDriver) Start() error {
	var err error
	var output string

	d.op_mtx.Lock()
	defer d.op_mtx.Unlock()

	if d.st == CAMERA_DRIVER_STATE_ON {
		return ErrNotStartable
	}

	drv_ins := d.opt.Sub("inputs")
	drv_outs := d.opt.Sub("outputs")
	fw := d.opt.Sub("framework")

	for _, k := range drv_ins.NextKeys() {
		drv_in := drv_ins.Sub(k)

		val := drv_in.GetString("file")
		if val == "" {
			return new_invalid_config_error(fmt.Sprintf("framework.inputs.%v.file", k))
		}

		drv_in_k := "inputs." + k
		fw.Set(drv_in_k, fw.Get(drv_in_k))
		fw.Set(fmt.Sprintf("inputs.%v.file", k), val)

		// TODO(Peer): accept multi-inputs
		break
	}

	for _, k := range drv_outs.NextKeys() {
		drv_out := drv_outs.Sub(k)

		val := drv_out.GetString("file_prefix")
		if val == "" {
			return new_invalid_config_error(fmt.Sprintf("framework.outputs.%v.file_prefix", k))
		}

		u, err := url.Parse(val + "/" + random_strings(64))
		if err != nil {
			return err
		}
		u.Path = path.Clean(u.Path)

		drv_out_k := "outputs." + k
		fw.Set(drv_out_k, fw.Get(drv_out_k))

		// TODO(Peer): accpet multi-outputs
		output = u.String()
		fw.Set(fmt.Sprintf("outputs.%v.file", k), output)

		break
	}

	fw_opt := &FrameworkOption{fw.Viper}
	d.frmwrk, err = NewFramework(fw.GetString("name"), fw_opt, "logger", d.logger)
	if err != nil {
		return err
	}

	err = d.frmwrk.Start()
	if err != nil {
		return err
	}

	err = d.mdl.PutObjects(map[string]io.Reader{
		"rtmp":  strings.NewReader(output),
		"state": strings.NewReader("on"),
	})
	if err != nil {
		return err
	}

	d.st = CAMERA_DRIVER_STATE_ON
	go func() {
		err := <-d.frmwrk.Wait()

		d.op_mtx.Lock()
		defer d.op_mtx.Unlock()

		if d.st == CAMERA_DRIVER_STATE_OFF {
			return
		}

		if err != nil {
			d.logger.WithError(err).Warningf("failed to wait framework")
		}

		d.reset()
	}()

	return nil
}

func (d *SimpleCameraDriver) Reset() {
	d.op_mtx.Lock()
	defer d.op_mtx.Unlock()

	d.reset()
}

func (d *SimpleCameraDriver) reset() {
	var err error

	err = d.mdl.RemoveObject("rtmp")
	if err != nil {
		d.logger.WithError(err).Warningf("failed to remove rtmp object")
	}

	err = d.mdl.PutObject("state", strings.NewReader("off"))
	if err != nil {
		d.logger.WithError(err).Warningf("failed to write off state")
	}

	d.frmwrk = nil
	d.st = CAMERA_DRIVER_STATE_OFF
}

func (d *SimpleCameraDriver) Stop() error {
	d.op_mtx.Lock()
	defer d.op_mtx.Unlock()

	if d.st == CAMERA_DRIVER_STATE_OFF {
		return ErrNotStoppable
	}

	err := d.frmwrk.Stop()
	if err != nil {
		d.logger.WithError(err).Debugf("failed to stop camera in framework")
	}

	d.reset()

	return nil
}

func (d *SimpleCameraDriver) State() *CameraDriverState {
	d.op_mtx.Lock()
	defer d.op_mtx.Unlock()

	return d.st
}

func NewSimpleCameraDriver(opt *CameraDriverOption, args ...interface{}) (CameraDriver, error) {
	var logger log.FieldLogger
	var module *component.Module

	opt_helper.Setopt(map[string]func(key string, val interface{}) error{
		"logger": opt_helper.ToLogger(&logger),
		"module": component.ToModule(&module),
	})(args...)

	drv := &SimpleCameraDriver{
		op_mtx: new(sync.Mutex),
		logger: logger,
		mdl:    module,
		opt:    opt,
		st:     CAMERA_DRIVER_STATE_OFF,
	}
	drv.Reset()

	return drv, nil
}

var register_simple_camera_driver_once sync.Once

func init() {
	register_simple_camera_driver_once.Do(func() {
		register_camera_driver_factory("simple", NewSimpleCameraDriver)
	})
}
