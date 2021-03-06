package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	term "github.com/appscode/go-term"
	pb "github.com/dinesh/datacol/api/models"
	"github.com/dinesh/datacol/cmd/provider/gcp"
	"github.com/dinesh/datacol/cmd/stdcli"
	"gopkg.in/urfave/cli.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var (
	credNotFound    = errors.New("Invalid credentials")
	projectNotFound = errors.New("Invalid project id")
)

func init() {
	stdcli.AddCommand(&cli.Command{
		Name:   "init",
		Usage:  "create new stack",
		Action: cmdStackCreate,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "stack",
				Usage: "Name of stack",
				Value: "demo",
			},
			&cli.StringFlag{
				Name:  "zone",
				Usage: "GCP zone for stack",
				Value: "us-east1-b",
			},
			&cli.StringFlag{
				Name:  "bucket",
				Usage: "GCP storage bucket",
			},
			&cli.IntFlag{
				Name:  "nodes",
				Usage: "number of nodes in container cluster",
				Value: 2,
			},
			&cli.StringFlag{
				Name:  "cluster",
				Usage: "name for existing Kubernetes cluster in GCP",
			},
			&cli.IntFlag{
				Name:  "disk-size",
				Usage: "SSD disk size for cluster in GB",
				Value: 10,
			},
			&cli.StringFlag{
				Name:  "machine-type",
				Usage: "name of machine-type to use for cluster",
				Value: "n1-standard-1",
			},
			&cli.BoolFlag{
				Name:  "preemptible",
				Usage: "use preemptible vm",
				Value: true,
			},
			&cli.BoolFlag{
				Name:  "opt-out",
				Usage: "Opt-out from getting updates via email from `datacol`",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "password",
				Usage: "api password for the stack",
			},
			&cli.StringFlag{
				Name:  "cluster-version",
				Usage: "The Kubernetes version to use for the master and nodes",
				Value: "1.6.4",
			},
		},
	})

	stdcli.AddCommand(&cli.Command{
		Name:   "destroy",
		Usage:  "destroy a stack from GCP",
		Action: cmdStackDestroy,
	})
}

func cmdStackCreate(c *cli.Context) error {
	stackName := c.String("stack")
	zone := c.String("zone")
	nodes := c.Int("nodes")
	bucket := c.String("bucket")
	password := c.String("password")

	cluster := c.String("cluster")
	machineType := c.String("machine-type")
	preemptible := c.Bool("preemptible")
	diskSize := c.Int("disk-size")

	message := `Welcome to Datacol CLI. This command will guide you through creating a new infrastructure inside your Google account. 
It uses various Google services (like Container engine, Cloudbuilder, Deployment Manager etc) under the hood to 
automate all away to give you a better deployment experience.

Datacol CLI will authenticate with your Google Account and install the Datacol platform into your GCP account. 
These credentials will only be used to communicate between this installer running on your computer and the Google platform.`

	fmt.Printf(message)
	prompt("")

	options := &gcp.InitOptions{
		Name:           stackName,
		ClusterName:    cluster,
		DiskSize:       diskSize,
		NumNodes:       nodes,
		MachineType:    machineType,
		Zone:           zone,
		Bucket:         bucket,
		Preemptible:    preemptible,
		Version:        stdcli.Version,
		ApiKey:         password,
		ClusterVersion: c.String("cluster-version"),
	}

	if err := initialize(options, nodes, c.Bool("opt-out")); err != nil {
		return err
	}

	term.Successln("\nDONE")

	fmt.Printf("Next, create an app with `STACK=%s datacol apps create`.\n", stackName)
	return nil
}

func cmdStackDestroy(c *cli.Context) error {
	if err := teardown(); err != nil {
		return err
	}

	term.Successln("\nDONE")
	return nil
}

func initialize(opts *gcp.InitOptions, nodes int, optout bool) error {
	resp := gcp.CreateCredential(opts.Name, optout)
	if resp.Err != nil {
		return resp.Err
	}

	cred := resp.Cred
	if len(cred) == 0 {
		return credNotFound
	}

	if len(resp.ProjectId) == 0 {
		return projectNotFound
	}

	if err := saveCredential(opts.Name, cred); err != nil {
		return err
	}

	opts.Project = resp.ProjectId
	opts.ProjectNumber = resp.PNumber
	opts.SAEmail = resp.SAEmail

	if len(opts.Bucket) == 0 {
		opts.Bucket = fmt.Sprintf("datacol-%s", slug(opts.Project))
	}

	name := opts.Name
	if len(opts.ClusterName) == 0 {
		opts.ClusterNotExists = true
		opts.ClusterName = fmt.Sprintf("%v-cluster", name)
	} else {
		opts.ClusterNotExists = false
	}

	apis := []string{
		"datastore.googleapis.com",
		"cloudbuild.googleapis.com",
		"deploymentmanager",
		"iam.googleapis.com",
	}

	url := fmt.Sprintf("https://console.cloud.google.com/flows/enableapi?apiid=%s&project=%s", strings.Join(apis, ","), opts.Project)

	fmt.Printf("\nDatacol needs to communicate with various APIs provided by cloud platform, please enable APIs by opening following link in browser and click Continue: \n%s\n", url)
	term.Confirm("Are you done ?")

	res, err := gcp.InitializeStack(opts)
	if err != nil {
		return err
	}

	fmt.Printf("\nStack hostIP %s\n", res.Host)
	fmt.Printf("Stack password: %s [Please keep is secret]\n", res.Password)
	fmt.Println("The above configuration has been saved in your home directory at ~/.datacol/config.json")

	return dumpParams(opts.Name, opts.Project, opts.Bucket, res.Host, res.Password)
}

func teardown() error {
	auth, rc := stdcli.GetAuthOrDie()
	if err := gcp.TeardownStack(auth.Name, auth.Project, auth.Bucket); err != nil {
		return err
	}

	if err := rc.DeleteAuth(); err != nil {
		return err
	}

	return os.RemoveAll(filepath.Join(pb.ConfigPath, auth.Name))
}

func createStackDir(name string) error {
	cfgroot := filepath.Join(pb.ConfigPath, name)
	return os.MkdirAll(cfgroot, 0700)
}

func saveCredential(name string, data []byte) error {
	if err := createStackDir(name); err != nil {
		return err
	}
	path := filepath.Join(pb.ConfigPath, name, pb.SvaFilename)
	log.Debugf("saving GCP credentials at %s", path)

	return ioutil.WriteFile(path, data, 0777)
}

func dumpParams(name, project, bucket, host, api_key string) error {
	auth := &stdcli.Auth{
		Name:      name,
		Project:   project,
		Bucket:    bucket,
		ApiServer: host,
		ApiKey:    api_key,
	}

	return stdcli.SetAuth(auth)
}
