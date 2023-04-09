package pgsnap

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// run postgres in docker and it will return the address to connect to
// and a function to call to stop the docker container
func runPostgresInDocker() (addr string, finish func() error, err error) {
	// pool are used to manage docker containers through the docker api / socket
	pool, err := dockertest.NewPool("")
	if err != nil {
		return "", nil, err
	}

	// create a postgres container
	resource, err := pool.RunWithOptions(getPostgreRunOptions("."))
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	finish = generateFinishFunction(pool, resource)

	// set the address to connect to
	addr = fmt.Sprintf(addrTmpl, resource.GetPort("5432/tcp"))

	err = waitUntilPostgresReady(pool, resource, addr)
	if err != nil {
		finish()
		return "", nil, err
	}

	return addr, finish, nil
}

func getPostgreRunOptions(migrationPath string) *dockertest.RunOptions {
	sqlMigrationPath, err := filepath.Abs(migrationPath)
	if err != nil {
		log.Fatal(err)
	}

	mount := sqlMigrationPath + ":/docker-entrypoint-initdb.d/"

	// postgres with latest tags
	return &dockertest.RunOptions{
		Repository: "postgres",
		Env:        []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
		Mounts:     []string{mount},
		//Tag:        "12",
	}
}

func generateFinishFunction(pool *dockertest.Pool, resource *dockertest.Resource) func() error {
	return func() error {
		// When you're done, kill and remove the container
		err := pool.Purge(resource)
		if err != nil {
			if errors.Is(err, &docker.NoSuchContainer{}) {
				return nil
			}
			log.Printf("Could not purge resource: %s", err)
		}
		return err
	}
}

func waitUntilPostgresReady(pool *dockertest.Pool, resource *dockertest.Resource, addr string) error {
	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	retryNum := 0

	err := pool.Retry(func() error {
		retryNum++

		db, err := sql.Open("postgres", addr)
		if err != nil {
			return err
		}
		defer db.Close()

		return db.Ping()
	})

	if err != nil {
		return fmt.Errorf("Could not connect to database after %d times: %s", retryNum, err)
	}

	return nil
}
