# app

An example of how to use sshtunnel to query a remote postgres database securely.

## Running

1. CLI args

    go run main.go --ssh.user sshuser --ssh.address cloudbox:22 --pg.user postgres --pg.password secret

2. A [config.yaml](config.example.yaml) file

    go run main.go
