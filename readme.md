# Cupboard

> A cupboard taking and putting containers freely.
 
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
        // the local port binding container 
	    BindingPort: "37379",
	}

	// create a container with option and return the information of the container
	// 
	// images will be pulled if don't exist and containers will be created for your using
	ret, cancel, err := WithContainer(context.Background(), opt)
	if err != nil {
		panic(err)
	}
	// the container will be deleted when error occur
	// 
	// the cancel function will delete the containers you created, else the container will be always live.
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
			// override container when the name of the container is duplicated
			Override:    true,
		},
		{
			Image:       "redis:latest",
			ExposedPort: "6379",
			Name:        "redis-2",
			Override:    true,
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
