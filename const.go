package cupboard

// default host IP
var hostIP = "127.0.0.1"

type Option struct {
	Override    bool     // Override container when the name of the container is duplicated
	Name        string   // The container name
	Image       string   // The container image and tag; eg: redis:latest
	ExposedPort string   // The exposed port of the container
	BindingPort string   // The port of the host binding the container; if not provide, the cupboard will generate a port randomly
	Protocol    string   // The protocol of connection; default is tcp
	Env         []string // List of environment variable to set in the container
	HostIP      string   // Host IP; default is 127.0.0.1
}

type payload struct {
	Info   *Info  // the information of containers
	Cancel func() // close the container running
	Index  int    // the index of options
}

type Info struct {
	Host        string // The host IP; eg: 127.0.0.1
	BindingPort string // The The port of the host binding the container
	URI         string // The URI to connect the container; eg: 127.0.0.1:2017
}

type Result struct {
	Infos []*Info // the information of containers
	Close func()  // close the containers running
	Wait  func()  // block and listen IOStreams close signal
}
