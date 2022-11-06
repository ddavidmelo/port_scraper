package main

import (
	"port_scraper/internal/config"
	"port_scraper/internal/scraper"
	"port_scraper/internal/sqldb"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	//Load configs
	config.Init_config()

	//Init storage
	sqldb.ConnectDB()
	sqldb.InitScanTable()
	sqldb.InitStaticTables()

	//Start Service
	scraper.Start()
}
