package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/mittwald/kube-httpcache/pkg/watcher"
)

func (v *VarnishController) Run(ctx context.Context) error {
	glog.Infof("waiting for initial configuration before starting Varnish")

	v.frontend = watcher.NewEndpointConfig()
	if v.frontendUpdates != nil {
		v.frontend = <-v.frontendUpdates
		if v.varnishSignaller != nil {
			v.varnishSignaller.SetEndpoints(v.frontend)
		}
	}

	v.backend = watcher.NewEndpointConfig()
	if v.backendUpdates != nil {
		v.backend = <-v.backendUpdates
	}

	target, err := os.Create(v.configFile)
	if err != nil {
		return err
	}

	glog.Infof("creating initial VCL config")
	err = v.renderVCL(target, v.frontend.Endpoints, v.frontend.Primary, v.backend.Endpoints, v.backend.Primary)
	if err != nil {
		return err
	}

	cmd, errChan := v.startVarnish(ctx)

	if err := v.waitForAdminPort(ctx); err != nil {
		return err
	}

	watchErrors := make(chan error)
	go v.watchConfigUpdates(ctx, cmd, watchErrors)

	go func() {
		for err := range watchErrors {
			if err != nil {
				glog.Warningf("error while watching for updates: %s", err.Error())
				retry(v, ctx)
			}
		}
	}()

	return <-errChan
}

func (v *VarnishController) startVarnish(ctx context.Context) (*exec.Cmd, <-chan error) {
	c := exec.CommandContext(
		ctx,
		"varnishd",
		v.generateArgs()...,
	)

	c.Dir = "/"
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	r := make(chan error)

	go func() {
		err := c.Run()
		r <- err
	}()

	return c, r
}

func (v *VarnishController) generateArgs() []string {
	args := []string{
		"-F",
		"-f", v.configFile,
		"-S", v.SecretFile,
		"-s", fmt.Sprintf("Cache=%s", v.Storage),
		"-s", fmt.Sprintf("Transient=%s", v.TransientStorage),
		"-a", fmt.Sprintf("%s:%d", v.FrontendAddr, v.FrontendPort),
		"-T", fmt.Sprintf("%s:%d", v.AdminAddr, v.AdminPort),
	}

	if v.AdditionalParameters != "" {
		for _, val := range strings.Split(v.AdditionalParameters, ",") {
			args = append(args, "-p")
			args = append(args, val)
		}
	}

	if v.WorkingDir != "" {
		args = append(args, "-n", v.WorkingDir)
	}

	return args
}

func retry(v *VarnishController, ctx context.Context) {
	for v.Attempt = 1; v.Attempt <= v.MaxRetries; v.Attempt++ {
		retryTime := v.RetryBackoff * time.Duration(v.Attempt)
		glog.Infof("retrying in %s, attempt %d/%d", retryTime, v.Attempt, v.MaxRetries)
		time.Sleep(retryTime)
		err := v.rebuildConfig(ctx)
		if err != nil {
			glog.Infof("error received: %s", err)
		}
	}
}
