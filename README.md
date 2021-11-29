# gomigrator
This package contains a very small MySQL migration solution for go written in 1 file.  
The aim of this package is to be very small but highly functional.

## Roadmap
  1. Add testing
  1. Whatever issue/idea comes up...

## Migration file structure
The migrations are regular SQL files containing sql statements.  
The file names need to be structured in the following way:
```bash
<version>_<name>.sql
```
Where the version is always an integer, incremented from the last version.  
The name is the name of the migration and preferably describes it.  
**The name may not contain an underscore (_) character.**

Example folder structure: 
- migrations/
  - 1_create-user-table.sql
  - 2_update-email-column.sql

## Example migration
Here is a very short migration example, just straight forward SQL like we all know it:
```sql
CREATE TABLE user (
  id                BINARY(16)    NOT NULL,
  email             VARCHAR(255)  NOT NULL,
);

ALTER TABLE user ADD CONSTRAINT unique_email UNIQUE(email);
ALTER TABLE user ADD CONSTRAINT pk_user PRIMARY KEY(id);
```

## Usage in an example API
Here is an example of a main file in an API, the essence is shown in the `if cfg.Mode == config.MODE_MIGRATE` part:
```go
package main

import (
  // Golog is a private logging package
  "github.com/reefercoding/golog"
  // This package
  "github.com/reefercoding/gomigrator"

  // Example internal packages
  "github.com/reefercoding/example-gomigrator/internal/config"
  "github.com/reefercoding/example-gomigrator/internal/server"
  "github.com/reefercoding/example-gomigrator/internal/storage"
)

func main() {
  // Example program config generation
  cfg := config.New()
  // Create *sql.DB
  db, err := storage.NewMysql(&cfg.MySQLConfig)
  if err != nil {
    golog.Fatalf("could not connect to mysql: %w", err)
  }
  if cfg.Mode == config.MODE_MIGRATE {
    // Stub logger if you like
    gomigrator.InfoLogger = golog.Infof
    err := gomigrator.Migrate(db, cfg.MigratorDir)
    if err != nil {
      golog.Fatalf("error occured during gomigrator execution: %w", err)
    }
  }
  if cfg.Mode == config.MODE_API {
    s := server.New(cfg, db)
    golog.Infof("starting http server on address...")
    forever := make(chan bool)
    go func() {
      if err := http.ListenAndServe(cfg.ListenAddress, s.Router); err != nil {
        golog.Fatalf("http server crashed: %w", err)
      }
    }()
    golog.Infof("successfully started http server, listening on: %s", cfg.ListenAddress)
    <-forever
  }
  golog.Infof("exiting application")
}
```