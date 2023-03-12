package main

import (
	"database/sql"
	"log"

	"github.com/gin/simplebank/api"
	db "github.com/gin/simplebank/db/sqlc"
	"github.com/gin/simplebank/utils"
	_ "github.com/lib/pq"
)

func main() {
	config, err := utils.LoadConfig(".")
	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal("can not connect: ", err)
	}

	store := db.NewStore(conn)
	server := api.NewServer(store)

	err = server.Start(config.ServerAddress)

	if err != nil {
		log.Fatalf("Can not connect %s, err: %s", config.ServerAddress, err)
	}
}
