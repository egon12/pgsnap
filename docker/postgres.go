package docker

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	dockertest "github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/cenkalti/backoff/v4"
)

const addrTmpl = "postgres://postgres@127.0.0.1:%s/?sslmode=disable"

type (

	// PostgreInDocker is an interface to manage a postgres container
	PostgreInDocker interface {
		// GetAddr returns the address to connect to
		GetAddr() string

		// Finish will stop the container
		Finish() error

		// GetLogs returns the logs of the container. It is useful for debugging
		GetLogs() string

		// WaitUntilReady will wait until the container is ready to accept connections
		WaitUntilReady() error
	}

	postgreInDocker struct {
		pool     *dockertest.Pool
		resource *dockertest.Resource
		isDebug  bool
		addr     string
		logs     *strings.Builder
	}
)

func NewPostgreInDocker(cfg PostgresConfig) (PostgreInDocker, error) {
	var err error
	p := &postgreInDocker{isDebug: cfg.DebugMode, logs: &strings.Builder{}}

	p.pool, err = dockertest.NewPool(cfg.DockerEndpoint)
	if err != nil {
		return p, fmt.Errorf("cannot connect to docker endpoint (%s) %w", cfg.DockerEndpoint, err)
	}

	option := p.generatePostgreOption(cfg)
	p.resource, err = p.pool.RunWithOptions(option, func(dcfg *docker.HostConfig) {
		dcfg.AutoRemove = !cfg.KeepContainer
	})
	if err != nil {
		return p, fmt.Errorf("cannot run container (%s) %w", option.Name, err)
	}

	if p.isDebug {
		log.Println("getting docker container logs")
	}
	_, err = p.pool.Client.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    p.resource.Container.ID,
		Stdout:       true,
		Stderr:       false,
		Stream:       true,
		Logs:         true,
		OutputStream: p.logs,
		ErrorStream:  p.logs,
	})
	if err != nil {
		log.Printf("Could not attach to container: %v", err)
	}

	p.addr = fmt.Sprintf(addrTmpl, p.resource.GetPort("5432/tcp"))
	if p.isDebug {
		log.Println("set the postgre addr into:", p.addr)
	}

	if !cfg.ExplicitWait {
		if p.isDebug {
			log.Println("wait until docker container ready")
		}
		err = p.WaitUntilReady()
		if err != nil {
			return p, fmt.Errorf("cannot wait until ready: %w", err)
		}
	}

	return p, nil
}

func (p *postgreInDocker) GetAddr() string {
	return p.addr
}

func (p *postgreInDocker) Finish() error {
	if p.isDebug {
		log.Printf("docker logs:\n%s\n", p.logs.String())
	}

	// When you're done, kill and remove the container
	err := p.pool.Purge(p.resource)
	if err != nil {
		if errors.Is(err, &docker.NoSuchContainer{}) {
			return nil
		}
		log.Printf("Could not purge resource: %s", err)
	}
	return err
}

func (p *postgreInDocker) GetLogs() string {
	return p.logs.String()
}

func (p *postgreInDocker) WaitUntilReady() error {
	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	retryNum := 0

	err := p.pool.Retry(func() error {
		retryNum++

		container, err := p.pool.Client.InspectContainer(p.resource.Container.ID)
		if err != nil {
			if p.isDebug {
				log.Printf("could not inspect container")
			}
		}

		if err == nil && container != nil {
			if p.isDebug {
				log.Printf("container in resource:  %s\n", &p.resource.Container.State)
				log.Printf("container from inspect: %s\n", &container.State)
			}
			if !container.State.Running {
				return fmt.Errorf("container: %s\n%s", &container.State, p.logs)
			}
			if container.State.Dead {
				err := fmt.Errorf("container dead: %s", p.logs)
				return &backoff.PermanentError{Err: err}
			}
			if !container.State.Running && !container.State.FinishedAt.IsZero() {
				err := fmt.Errorf("container dead: %s", p.logs)
				return &backoff.PermanentError{Err: err}
			}
	
		}

		db, err := sql.Open("postgres", p.addr)
		if err != nil {
			if p.isDebug {
				log.Printf("could not open into %s\n: %v", p.addr, err)
			}
			return err
		}
		defer db.Close()

		err = db.Ping()
		if p.isDebug {
			log.Printf("ping err is %v\n", err)
		}
		return err
	})

	if err != nil {
		return fmt.Errorf("Could not connect to database after %d times: %s", retryNum, err)
	}

	return nil
}

func (p *postgreInDocker) generatePostgreOption(cfg PostgresConfig) *dockertest.RunOptions {
	migrationPath := p.getMigrationPath(cfg)

	if p.isDebug {
		log.Println("use migration path in:", migrationPath)
	}

	sqlMigrationPath, err := filepath.Abs(migrationPath)
	if err != nil {
		log.Fatal(err)
	}

	if p.isDebug {
		log.Println("use absolute migration path in:", sqlMigrationPath)
	}

	mount := sqlMigrationPath + ":/docker-entrypoint-initdb.d/"

	// postgres with latest tags
	return &dockertest.RunOptions{
		Repository: "postgres",
		Env:        []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
		Mounts:     []string{mount},
		Name:       p.getContainerName(cfg),
		Tag:        cfg.PostgresVersion,
	}
}

func (p *postgreInDocker) getMigrationPath(cfg PostgresConfig) string {
	if cfg.MigrationPath != "" {
		return cfg.MigrationPath
	}

	if migrationPath != "" {
		return migrationPath
	}

	return "."
}

func (p *postgreInDocker) getContainerName(cfg PostgresConfig) string {
	return "pgsnap_test" + cfg.ContainerNameSuffix
}
