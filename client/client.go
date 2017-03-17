package client 

import (
  "path/filepath"
  "os"
  "fmt"
  "errors"
  "encoding/json"

  "github.com/joyrexus/buckets"
  homedir "github.com/mitchellh/go-homedir"
  "github.com/dinesh/rz/cmd/stdcli"

  "github.com/dinesh/rz/cloud"
)

var (
  rzDirName  = ".razorbox"
  dbFileName = "rz.db"
  stack404 = errors.New("Please create a stack with: $ rz init")
)

func init() {
  home, err := homedir.Dir()
  if err != nil {
    stdcli.Error(err)
    return
  }

  root := filepath.Join(home, rzDirName)
  
  if _, err = os.Stat(root); err != nil {
    if !os.IsNotExist(err) {
      stdcli.Error(err)
      return
    } else {
      if err := os.MkdirAll(root, 0700); err != nil {
        stdcli.Error(err)
        return
      }
    }
  }

  dbpath := filepath.Join(home, rzDirName, dbFileName)
  db, err := buckets.Open(dbpath)
  if err != nil {
    stdcli.Error(fmt.Errorf("creating database file: %v", err))
    return
  }
  
  DB = db
}

func Persist(b []byte, pk string, object interface {}) error {
  bx, _ := DB.New(b)
  encoded, err := json.Marshal(object)
  if err != nil { return err }

  return bx.Put([]byte(pk), encoded)
}

type Client struct {
  Version     string
  StackName   string
  *Stack
}

func (c *Client) configRoot() string {
  home, _ := homedir.Dir()
  return filepath.Join(home, rzDirName, c.StackName)
}

func (c *Client) SetStack(name string) error {
  c.StackName = name
  st, err := FindStack(name)
  if err != nil { return stack404 }

  c.Stack = st
  return nil
}

func (c *Client) Provider() cloud.Provider {
  return cloud.Getgcp(
    c.Stack.Name,
    c.Stack.ProjectId, 
    c.Stack.Zone,
    c.Stack.Bucket, 
    c.Stack.ServiceKey,
  )
}
