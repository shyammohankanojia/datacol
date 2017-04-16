package main

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v2"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/dinesh/datacol/client/models"
	"github.com/dinesh/datacol/cmd/stdcli"
)

func init() {
	stdcli.AddCommand(cli.Command{
		Name:      "run",
		UsageText: "execute a command in an app",
		Action:    cmdAppRun,
	})
}

func cmdAppRun(c *cli.Context) error {
	_, name, err := getDirApp(".")
	if err != nil {
		return err
	}

	ctl := getClient(c)
	if _, err := ctl.GetApp(name); err != nil {
		return err
	}

	pod, err := ctl.Provider().GetRunningPods(name)
	if err != nil {
		return err
	}

	excode := execute_exec(ctl.Stack.Name, pod, c.Args())
	os.Exit(excode)
	return nil
}

func execute_exec(env, pod string, args []string) int {
	var (
		out, outErr bytes.Buffer
		exitcode    int
	)

	cfgpath := filepath.Join(models.ConfigPath, env, "kubeconfig")
	args = append([]string{"--kubeconfig", cfgpath, "-n", env, "--pod", pod, "exec"}, args...)
	c := exec.Command("kubectl", args...)

	log.Debugf("kubectl %+v", args)

	c.Stdout = &out
	c.Stderr = &outErr
	err := c.Run()

	if exitError, ok := err.(*exec.ExitError); ok {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			exitcode = waitStatus.ExitStatus()
		}
	}
	if exitcode == 0 {
		fmt.Printf(out.String())
	} else {
		fmt.Printf(outErr.String())
	}

	return exitcode
}
