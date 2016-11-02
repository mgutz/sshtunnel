package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	conf "github.com/mgutz/configpipe"
	"github.com/mgutz/sshtunnel"

	"github.com/mgutz/dat/dat"
	runner "github.com/mgutz/dat/sqlx-runner"

	"golang.org/x/crypto/ssh"
)

var config *conf.Configuration

func init() {
	var err error
	config, err = conf.Runv(
		conf.YAMLFile(&conf.File{Path: "./config.yaml"}),
		conf.Argv(),
		//conf.Trace(),
	)
	if err != nil {
		panic(err)
	}
}

// open connection to postgres
func postgres(connstr string) (*runner.DB, error) {
	sqlDB, err := sql.Open("postgres", connstr)
	if err != nil {
		return nil, err
	}

	runner.MustPing(sqlDB)

	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(2)

	dat.Strict = true
	dat.EnableInterpolation = true
	runner.LogQueriesThreshold = 20 * time.Millisecond
	runner.LogErrNoRows = false

	return runner.NewDB(sqlDB, "postgres"), nil
}

// Open database connection using environment variable credentials.
func openDatabase(user string, password string) (*runner.DB, error) {
	connstr := fmt.Sprintf("user=%s password=%s dbname=mno_production host=127.0.0.1 port=25432 sslmode=disable", user, password)
	return postgres(connstr)
}

// Get the count of users from production database.
func query(conn runner.Connection) error {
	var count int
	conn.SQL(`select count(*) from users`).QueryScalar(&count)
	fmt.Println("COUNT", count)
	return nil
}

func main() {
	sshConfig := &ssh.ClientConfig{
		User: config.MustString("ssh.user"),
		Auth: []ssh.AuthMethod{
			sshtunnel.SSHAgent(),
		},
	}

	tunnelConf := sshtunnel.Config{
		SSHAddress:    config.MustString("ssh.address"),
		RemoteAddress: "127.0.0.1:5432",
		LocalAddress:  "127.0.0.1:25432",
		SSHConfig:     sshConfig,
	}

	tunnel := sshtunnel.New(&tunnelConf)

	if err := <-tunnel.Open(); err != nil {
		tunnel.Close()
		panic(err)
	}

	// TODO is there a more elegant way to do clean up in tunnel itself if the program
	// is terminated, need to close database connections as well
	cleanup := func() {
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			tunnel.Close()
			os.Exit(1)
		}()
	}

	go cleanup()

	db, err := openDatabase(config.MustString("pg.user"), config.MustString("pg.password"))
	if err != nil {
		panic(err)
	}

	err = query(db)
	if err != nil {
		fmt.Println(err)
	}
}
