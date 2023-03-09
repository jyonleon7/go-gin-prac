# postgres のデータベースを作成する
postgres:
	docker run --name postgres12 -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:12-alpine

# 作成した データベースに テーブルを作成する[simple_bank]
createdb:
	docker exec -it postgres12 createdb --username=root --owner=root simple_bank

# テーブルを削除する[simple_bank]
dropdb:
	docker exec -it postgres12 dropdb simple_bank

# テーブル作成のマイグレーションを実行する
migrateup:
	migrate --path db/migration -database "postgresql://root:secret@localhost:5432/simple_bank?sslmode=disable" --verbose up

# テーブルをドロップするマイグレーションを実行する
migratedown:
	migrate --path db/migration -database "postgresql://root:secret@localhost:5432/simple_bank?sslmode=disable" --verbose down

start:
	docker start postgres12

stop:
	docker stop postgres12

sqlc:
	sqlc generate

#  -v; 詳細な出力を得ることができる, -cover: コードのどの部分がテストされているかを示す、 ./... : カレントディレクトリは以下のすべてが対象
test:
	go test -v -cover ./...

server:
	go run main.go

.PHONY: postgres createdb dropdb migrateup migratedown start stop sqlc test server