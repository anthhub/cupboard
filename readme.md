# Cupboard

> A cupboard accessing to containers freely.
 
> A package controlling containers programmatically.


## Installation

```bash
go get github.com/anthhub/cupboard
```


## Usage

```go
	opt := &Option{
		// the container image and tag
		Image:       "mongo:4.4",
		// the exposed port of the container 
		ExposedPort: "27017",
	}

	// create a container with option and return the information of the container
	ret, cancel, err := WithContainer(context.Background(), opt)
	if err != nil {
		panic(err)
	}
	// the cancel function will delete the container you created, else the container will be not deleted.
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
	// ...
```

## Advanced Usage

### Multiple Containers

```go
	opt := []*Option{
		{
			// the container image and tag
			Image:       "redis:latest",
			// the exposed port of the container 
			ExposedPort: "6379",
			// the name of the container, it has to be unique, else panic will occur.
			Name:        "redis-1",
		},
		{
			Image:       "redis:latest",
			ExposedPort: "6379",
			Name:        "redis-2",
		},
	}

	// create multiple containers.
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
		if err != nil {
			return
		}
		// ...
	}
```

> If you want to learn more about `cupboard`, you can read test cases and source code.
