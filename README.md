##### openshift-controller-manager

TODO: Add content

## Development

To get started, [fork](https://help.github.com/articles/fork-a-repo) the [repo](https://github.com/openshift/openshift-controller-manager)

### Develop locally on your host

#### Installing Prerequisites

##### MacOS

Follow the installation steps to install [Homebrew](http://brew.sh), which will
allow you to install the following build dependencies:
```
$ brew install gpgme 
```

#### Building and testing the binary

In order to build openshift-controller-manager run:
```
$ make build
```

To run the unit tests type:
```
$ make test-unit
```

To list all available targets, run: 
```
$ make help
```

#### Setting up a local env for debugging purposes
In order to run the binary locally you need to have a valid configuration file, run the following script to create one in `dev_env` directory
```
$ ./hack/dev_env.sh
```

Next use a tool of your choice to start the process in debug mode. If you happen to be Goland user you will need a [debug configuration](https://www.jetbrains.com/help/go/creating-and-editing-run-debug-configurations.html) with the following settings:
```
Program arguments: start --alsologtostderr -v=1 --config dev_env/config.json
Working directory: $GOPATH/src/github.com/openshift/openshift-controller-manager
```
