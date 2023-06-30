package main

import (
	"Backend/config"
	"Backend/routers"
)

func main() {
	db, err := config.SetupMySQLConnection()
	if err != nil {
		panic("無法連接到資料庫")
	}
	defer func() {
		dbInstance, _ := db.DB()
		_ = dbInstance.Close()
	}()

	rdb, err := config.SetupRedisConnection()
	if err != nil {
		panic("無法連接到Redis")
	}
	defer rdb.Close()

	router := routers.SetupRouters(db, rdb)
	router.Run(":3000")
}
