package sqldb

import (
	"database/sql"
	"fmt"
	"port_scraper/internal/config"
	"time"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

var database *DBHandler

type DBHandler struct {
	SQL *sql.DB
}

// ConnectDB opens a connection to the database
func ConnectDB() {
	log.Info("--- Connecting to DB")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true&timeout=5s", config.GetDBEnv().User, config.GetDBEnv().Password, config.GetDBEnv().Host, config.GetDBEnv().Port)
	d, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer d.Close()

	for {
		if err := d.Ping(); err != nil {
			log.Error("storage: ping mySQL database error, will retry in 5s")
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	_, err = d.Exec("CREATE DATABASE IF NOT EXISTS " + config.GetDBEnv().Name)
	if err != nil {
		panic(err)
	}
	d.Close()

	dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=5s", config.GetDBEnv().User, config.GetDBEnv().Password, config.GetDBEnv().Host, config.GetDBEnv().Port, config.GetDBEnv().Name)
	d, err = sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}

	d.SetConnMaxIdleTime(1 * time.Second)
	d.SetConnMaxLifetime(30 * time.Second)

	database = &DBHandler{d}
	log.Info("--- Connected to DB")

}

func ping(dbb *sql.DB) {
	err := dbb.Ping()

	if err != nil {
		panic(err)
	}
}

func Exec(db *DBHandler, query string, args ...interface{}) (sql.Result, error) {
	return db.SQL.Exec(query, args...)
}

func Query(db *DBHandler, query string, args ...interface{}) (*sql.Rows, error) {
	return db.SQL.Query(query, args...)
}

func QueryRow(db *DBHandler, query string, args ...interface{}) *sql.Row {
	return db.SQL.QueryRow(query, args...)
}

func DB() *DBHandler {
	return database
}

func ClearTable(name string) error {
	_, err := Exec(DB(), "DELETE FROM "+name)
	return err
}
