package cupboard

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"gopkg.in/mgo.v2/bson"
)

func TestRedis(t *testing.T) {

	opt := []*Option{
		{
			Image:       "redis:latest",
			ExposedPort: "6379",
			Name:        "redis-1",
			Override:    true,
			BindingPort: "36379",
		},
		{
			Image:       "redis:latest",
			ExposedPort: "6379",
			Name:        "redis-2",
			Override:    true,
			BindingPort: "37379",
		},
	}

	rets, cancel, err := WithContainers(context.Background(), opt)
	if err != nil {
		panic(err)
	}
	defer cancel()

	for _, ret := range rets {

		client := redis.NewClient(&redis.Options{
			Addr:       ret.URI,
			Password:   "",
			DB:         0,
			MaxRetries: 10,
		})

		var (
			r string
			n = "testing"
		)
		_, err = client.Ping().Result()
		assert.NoError(t, err)
		if err != nil {
			return
		}

		_, err = client.Set("one", n, time.Second).Result()
		assert.NoError(t, err)

		r, err = client.Get("one").Result()
		assert.NoError(t, err)

		assert.Equal(t, n, r)
	}
}

func TestMongo(t *testing.T) {

	opt := &Option{
		Image:       "mongo:4.4",
		ExposedPort: "27017",
		BindingPort: "37017",
	}

	ret, cancel, err := WithContainer(context.Background(), opt)
	if err != nil {
		panic(err)
	}
	defer cancel()

	c := context.Background()
	client, err := mongo.Connect(c, options.Client().ApplyURI("mongodb://"+ret.URI))
	assert.NoError(t, err)
	assert.NotNil(t, client)

	ctx, cancel := context.WithTimeout(c, time.Minute)
	defer cancel()
	err = client.Ping(ctx, readpref.Primary())
	assert.NoError(t, err)
	if err != nil {
		return
	}

	col := client.Database("testing_db").Collection("testing_col")

	id, err := primitive.ObjectIDFromHex("5f7c245ab0361e00ffb9fd6f")
	assert.NoError(t, err)

	id2, err := primitive.ObjectIDFromHex("5f7c245ab0361e00ffb9fd6e")
	assert.NoError(t, err)

	var (
		field       = "field"
		oid         = "_id"
		fieldValue  = "testing_field"
		fieldValue2 = "testing_field2"
	)

	type Row struct {
		ID    primitive.ObjectID `bson:"_id"`
		Field string             `bson:"field"`
	}
	var row Row

	// c
	_, err = col.InsertMany(c, []interface{}{
		bson.M{
			oid:   id,
			field: fieldValue,
		},
	})
	assert.NoError(t, err)

	// r
	res := col.FindOne(c, bson.M{
		field: fieldValue,
	})
	err = res.Err()
	assert.NoError(t, err)

	err = res.Decode(&row)
	assert.NoError(t, err)

	assert.Equal(t, row.ID, id)
	assert.Equal(t, row.Field, fieldValue)

	// u
	res = col.FindOneAndUpdate(c, bson.M{
		field: fieldValue2,
	},
		bson.M{
			"$setOnInsert": bson.M{
				oid:   id2,
				field: fieldValue2,
			},
		}, options.FindOneAndUpdate().
			SetUpsert(true).
			SetReturnDocument(options.After))

	err = res.Err()
	assert.NoError(t, err)

	err = res.Decode(&row)
	assert.NoError(t, err)

	assert.Equal(t, row.ID, id2)
	assert.Equal(t, row.Field, fieldValue2)

	// d
	_, err = col.DeleteOne(c, bson.M{
		field: fieldValue2,
	})
	assert.NoError(t, err)
}

func TestMysql(t *testing.T) {

	opt := &Option{
		Image:       "mysql:latest",
		ExposedPort: "3306",
		BindingPort: "33306",
		Env:         []string{"MYSQL_ALLOW_EMPTY_PASSWORD=yes", "USER=root", "MYSQL_DATABASE=demo"},
	}
	c := context.Background()
	rs, cancel, err := WithContainer(c, opt)
	if err != nil {
		panic(err)
	}
	defer cancel()

	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s", "root", "", rs.Host, rs.BindingPort, "demo", "utf8mb4")
	db, err := sql.Open("mysql", dbDSN)
	assert.NoError(t, err)

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

	assert.NoError(t, err)
	if err != nil {
		return
	}

	type User struct {
		Id   int64  `db:"id"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	var (
		name  = "小红"
		age   = 23
		name2 = "小黑"
		age2  = 10
		one   = int64(1)
	)
	// c
	ret, err := db.Exec("" +
		" CREATE TABLE IF NOT EXISTS `user`(" +
		" `id` INT UNSIGNED AUTO_INCREMENT," +
		" `name` VARCHAR(100) NOT NULL," +
		" `age` INT NOT NULL," +
		" PRIMARY KEY ( `id` )" +
		" )ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" +
		"")
	assert.NoError(t, err)
	assert.NotNil(t, ret)

	ret, err = db.Exec("INSERT INTO user(name, age) VALUES(?,?)", name, age)
	assert.NoError(t, err)
	assert.NotNil(t, ret)

	lastInsertID, _ := ret.LastInsertId()

	rowsaffected, _ := ret.RowsAffected()
	assert.Equal(t, rowsaffected, one)

	// r
	user := new(User)
	row := db.QueryRow("SELECT id, name, age FROM user WHERE id= ?", lastInsertID)
	assert.NoError(t, row.Err())

	err = row.Scan(&user.Id, &user.Name, &user.Age)
	assert.NoError(t, err)

	assert.Equal(t, user.Id, lastInsertID)
	assert.Equal(t, user.Name, name)
	assert.Equal(t, user.Age, age)

	// u
	ret, err = db.Exec(`UPDATE user SET name=?, age=? WHERE id= ?`, name2, age2, user.Id)
	assert.NoError(t, err)
	rowsaffected, _ = ret.RowsAffected()
	assert.Equal(t, rowsaffected, one)

	row = db.QueryRow("SELECT id, name, age FROM user WHERE id= ?", lastInsertID)
	assert.NoError(t, row.Err())
	err = row.Scan(&user.Id, &user.Name, &user.Age)
	assert.NoError(t, err)

	assert.Equal(t, user.Id, lastInsertID)
	assert.Equal(t, user.Name, name2)
	assert.Equal(t, user.Age, age2)

	// d
	ret, err = db.Exec("DELETE FROM user WHERE id = ?", lastInsertID)
	assert.NoError(t, err)
	rowsaffected, _ = ret.RowsAffected()
	assert.Equal(t, rowsaffected, one)
}
