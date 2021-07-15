package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthhub/cupboard"
	_ "github.com/go-sql-driver/mysql"
)

func prepareDB() {
	opt := &cupboard.Option{
		HostIP:      "0.0.0.0",
		Image:       "mysql:latest",
		ExposedPort: "3306",
		BindingPort: "33307",
		Env:         []string{"MYSQL_ALLOW_EMPTY_PASSWORD=yes", "USER=root", "MYSQL_DATABASE=demo"},
	}
	c := context.Background()
	rs, cancel, err := cupboard.WithContainer(c, opt)
	if err != nil {
		panic(err)
	}
	defer cancel()
	fmt.Printf("\n rs %v \n ", rs)

	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=true", "root", "", rs.Host, rs.BindingPort, "demo", "utf8mb4")
	db, err := sql.Open("mysql", dbDSN)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n dbDSN %v \n ", dbDSN)

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	ctx, cancel := context.WithTimeout(c, time.Minute)
	defer cancel()

	err = func() error {
		i := 0
		for {
			i++
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout")
			default:
				err = db.PingContext(ctx)
				if err == nil {
					return nil
				}
			}
			fmt.Printf("ping %d times per 2 second\n", i)
			time.Sleep(2 * time.Second)
		}
	}()

	if err != nil {
		panic(err)
	}

	prepareData(db)
}

func prepareData(db *sql.DB) {
	{
		_, err := db.Exec(" CREATE TABLE `pencil_files` ( " +
			" `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键, 无业务意义', " +
			" `name` varchar(255) NOT NULL COMMENT '文件名', " +
			" `lesson_type` tinytext COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'lesson_type 名称', " +
			" `audio_count` varchar(128) NOT NULL COMMENT '包含多少条短音频', " +
			" `created_at` datetime DEFAULT NULL COMMENT '创建时间', " +
			" `updated_at` datetime DEFAULT NULL COMMENT '更新时间', " +
			" PRIMARY KEY (`id`), " +
			" UNIQUE KEY `uk_lesson_type_name` (`lesson_type`(32), `name`(32)) " +
			" ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='pencil 文件记录'; ")

		if err != nil {
			panic(err)
		}
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("%s-%d", "qiyuan", i)
			_, err = db.Exec("INSERT INTO pencil_files(name,lesson_type,audio_count,created_at,updated_at) VALUES(?,'alix','2','2021-07-15 13:18:49','2021-07-15 13:18:51')", name)
			if err != nil {
				panic(err)
			}
		}
	}

	{
		_, err := db.Exec(" CREATE TABLE `pencil_audio` ( " +
			" `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键, 无业务意义', " +
			" `name` varchar(255) NOT NULL COMMENT 'audio name', " +
			" `oss_url` varchar(255) NOT NULL COMMENT 'oss url', " +
			" `cdn_url` varchar(255) NOT NULL COMMENT 'cdn url', " +
			" `file_id` bigint(20) NOT NULL COMMENT 'pencil_files id', " +
			" `file_name` varchar(255) NOT NULL COMMENT '文件名', " +
			" `text` varchar(255) NOT NULL COMMENT '对应文本', " +
			" `created_at` datetime DEFAULT NULL COMMENT '创建时间', " +
			" `updated_at` datetime DEFAULT NULL COMMENT '更新时间', " +
			" PRIMARY KEY (`id`), " +
			" UNIQUE KEY `uk_name` (`name`(32)), " +
			" KEY `idx_file_id` (`file_id`), " +
			" KEY `idx_file_name` (`file_name`(32)) " +
			" ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='pencil 音频记录'; ")

		if err != nil {
			panic(err)
		}
		for i := 0; i < 100; i++ {
			name := fmt.Sprintf("%s-%d", "qiyuan", i)
			_, err = db.Exec("INSERT INTO pencil_audio(name,oss_url,cdn_url,file_id,file_name,text,created_at,updated_at) VALUES(?,'','',2,'qiyuan-1',?,'2021-07-15 13:18:49','2021-07-15 13:18:51')", name, name)
			if err != nil {
				panic(err)
			}
		}
	}

	fmt.Println("wait...")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	fmt.Println("Bye...")
}

func main() {
	prepareDB()
}
