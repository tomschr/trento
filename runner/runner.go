package runner

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/trento-project/trento/internal"
	"github.com/trento-project/trento/internal/consul"

	"github.com/trento-project/trento/api"
)

//go:embed ansible
var ansibleFS embed.FS

const (
	AnsibleMain       = "ansible/check.yml"
	AnsibleMeta       = "ansible/meta.yml"
	AnsibleConfigFile = "ansible/ansible.cfg"
	AnsibleHostFile   = "ansible/ansible_hosts"
)

type Runner struct {
	config    *Config
	ctx       context.Context
	ctxCancel context.CancelFunc
	consul    consul.Client
	trentoApi api.TrentoApiService
}

type Config struct {
	ApiHost       string
	ApiPort       int
	AraServer     string
	Interval      time.Duration
	AnsibleFolder string
}

func NewRunner(config *Config) (*Runner, error) {
	client, err := consul.DefaultClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not create the consul client connection")
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	runner := &Runner{
		config:    config,
		ctx:       ctx,
		ctxCancel: ctxCancel,
		consul:    client,
	}

	return runner, nil
}

func (c *Runner) Start() error {
	var wg sync.WaitGroup

	if err := createAnsibleFiles(c.config.AnsibleFolder); err != nil {
		return err
	}

	trentoApi := api.NewTrentoApiService(c.config.ApiHost, c.config.ApiPort)
	if !trentoApi.IsWebServerUp() {
		return fmt.Errorf("Trento server api not available")
	}

	c.trentoApi = trentoApi

	metaRunner, err := NewAnsibleMetaRunner(c.config)
	if err != nil {
		return err
	}

	if !metaRunner.IsAraServerUp() {
		return fmt.Errorf("ARA server not available")
	}

	if err = metaRunner.RunPlaybook(); err != nil {
		return err
	}

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		log.Println("Starting the runner loop...")
		defer wg.Done()
		c.startCheckRunnerTicker()
		log.Println("Runner loop stopped.")
	}(&wg)

	wg.Wait()

	return nil
}

func (c *Runner) Stop() {
	c.ctxCancel()
}

func createAnsibleFiles(folder string) error {
	log.Infof("Creating the ansible file structure in %s", folder)
	// Clean the folder if it stores old files
	ansibleFolder := path.Join(folder, "ansible")
	err := os.RemoveAll(ansibleFolder)
	if err != nil {
		log.Error(err)
		return err
	}

	err = os.MkdirAll(ansibleFolder, 0755)
	if err != nil {
		log.Error(err)
		return err
	}

	// Create the ansible file structure from the FS
	err = fs.WalkDir(ansibleFS, "ansible", func(fileName string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !dir.IsDir() {
			content, err := ansibleFS.ReadFile(fileName)
			if err != nil {
				log.Errorf("Error reading file %s", fileName)
				return err
			}
			f, err := os.Create(path.Join(folder, fileName))
			if err != nil {
				log.Errorf("Error creating file %s", fileName)
				return err
			}
			fmt.Fprintf(f, "%s", content)
		} else {
			os.Mkdir(path.Join(folder, fileName), 0755)
		}
		return nil
	})

	if err != nil {
		log.Errorf("An error ocurred during the ansible file structure creation: %s", err)
		return err
	}

	log.Info("Ansible file structure successfully created")

	return nil
}

func NewAnsibleMetaRunner(config *Config) (*AnsibleRunner, error) {
	playbookPath := path.Join(config.AnsibleFolder, AnsibleMeta)
	ansibleRunner, err := DefaultAnsibleRunnerWithAra()
	if err != nil {
		return ansibleRunner, err
	}

	if err = ansibleRunner.SetPlaybook(playbookPath); err != nil {
		return ansibleRunner, err
	}

	configFile := path.Join(config.AnsibleFolder, AnsibleConfigFile)
	ansibleRunner.SetConfigFile(configFile)
	ansibleRunner.SetTrentoApiData(config.ApiHost, config.ApiPort)
	ansibleRunner.SetAraServer(config.AraServer)

	return ansibleRunner, err
}

func NewAnsibleCheckRunner(config *Config) (*AnsibleRunner, error) {
	playbookPath := path.Join(config.AnsibleFolder, AnsibleMain)

	ansibleRunner, err := DefaultAnsibleRunnerWithAra()
	if err != nil {
		return ansibleRunner, err
	}

	if err = ansibleRunner.SetPlaybook(playbookPath); err != nil {
		return ansibleRunner, err
	}

	ansibleRunner.Check = true
	configFile := path.Join(config.AnsibleFolder, AnsibleConfigFile)
	ansibleRunner.SetConfigFile(configFile)
	ansibleRunner.SetTrentoApiData(config.ApiHost, config.ApiPort)
	ansibleRunner.SetAraServer(config.AraServer)

	return ansibleRunner, nil
}

func (c *Runner) startCheckRunnerTicker() {
	checkRunner, err := NewAnsibleCheckRunner(c.config)
	if err != nil {
		return
	}

	tick := func() {
		content, err := NewClusterInventoryContent(c.consul, c.trentoApi)
		if err != nil {
			log.Errorf("Error creating the ansible inventory content")
			return
		}

		inventoryFile := path.Join(c.config.AnsibleFolder, AnsibleHostFile)
		err = CreateInventory(inventoryFile, content)
		if err != nil {
			log.Errorf("Error creating the ansible inventory file")
			return
		}

		if err = checkRunner.SetInventory(inventoryFile); err != nil {
			return
		}

		if !checkRunner.IsAraServerUp() {
			log.Error("ARA server not found. Skipping ansible execution as the data is not recorded")
			return
		}
		checkRunner.RunPlaybook()
	}

	interval := c.config.Interval
	internal.Repeat("runner.ansible_playbook", tick, interval, c.ctx)
}
