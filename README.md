# pgsnap
Fake postgre proxy for unit testing

## Getting Started
You can download the library in by typing this command

```
go get github.com/egon12/pgsnap
```
and then use it in test
```go
package db

import (
  "databse/sql"
  "testing"
  
  "github.com/egon12/pgsnap"
  "github.com/lib/pq"
)

func TestDB_GetProduct(t *testing.T) {
  snap := pgsnap.New(t, "postgresql://user:password@localhost:5432/dbname")
  defer snap.Finish()
  
  db, _ := sql.Open(snap.Addr())

  p := ProductRepo{DB: db}
  p, err := p.Get(1)

  if err != nil {
    t.Errorf("got error: %v", err)
  }

  if p.ID != 1 {
    t.Error("got wrong product")
  }
}

```

## Why we need this?
The best way to test postgre is by using real DB. why, usually what we pass are query
And the one that can predict query are the DB itself. But it comes with a large baggage.
Using DB as testing is quite hard, because we need to maintain the DB content while we 
do the testing.

The snapshot testing is inspired by snapshot testing UI using jest. In jest, to create the 
code at first we will use real UI (browser). And then when we do test, we will create some 
snapshot (tree structure of the UI). And everytime we do some test, it will try to mathced
with snapshot that we have.


## How it's work?
when we create pgsnap, we will create postgresql proxy, that will be used to get message that 
send/receive between app and postgres. At first run, or when we don't have the snapshot file,
it will create the snapshot file and save it.

At the second run the proxy wouldn't not connect to the real postgres server, and instead read
the snapshot file and response accordingly to the app.

### First run
```mermaid
graph LR

    TestApp --> pgsnap.Proxy --> postgreSQL_Server
    postgreSQL_Server --> pgsnap.Proxy --> TestApp
    pgsnap.Proxy --> snapshot_file
    
```

### Second run (and run in CI/CD environment)
```mermaid
graph LR

    TestApp --> pgsnap.Proxy --> snapshot_file
    snapshot_file --> pgsnap.Proxy --> TestApp
    
```

### Known Bugs
For now, we only support `github.com/lib/pq`. This caused by different implementaton in 
creating transaction statement. In `lib/pq` transacion is not named. But in jackc/pgx,
the transaction is named by incremental singleton value. So there are possibility 
first run and second run have different value. Depends on how many query already run 
in the test process.
