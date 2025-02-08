<div id="top"></div>

# BEAN (豆)

A web framework written in GO on-top of `echo` to ease your application development. Our main goal is not to compete with other `Go` framework instead we are mainly focusing on `tooling` and `structuring` a project to make developers life a bit easier and less stressful to maintain their project more in a lean way.

- [BEAN (豆)](#bean-豆)
  - [How to use](#how-to-use)
    - [Initialize a project](#initialize-a-project)
  - [Service-Repository Pattern](#service-repository-pattern)
  - [How To Create Repository File(s)](#how-to-create-repository-files)
  - [One Liner To Create Service And Repositories](#one-liner-to-create-service-and-repositories)
  - [How To Create Handler](#how-to-create-handler)
  - [Two Build Commands](#two-build-commands)
- [Additional Features](#additional-features)
  - [Built-In Logging](#built-in-logging)
  - [Built-In testing](#built-in-testing)
  - [Out of the Box Commands](#out-of-the-box-commands)
    - [Generating Secret Key using gen secret command](#generating-secret-key-using-gen-secret-command)
    - [Cryptography using the aes command](#cryptography-using-the-aes-command)
    - [Listing routes using the route list command](#listing-routes-using-the-route-list-command)
  - [Make your own Commands](#make-your-own-commands)
  - [Local K/V Memorystore](#local-kv-memorystore)
  - [Useful Helper Functions](#useful-helper-functions)
  - [Bean Config](#bean-config)
  - [TenantAlterDbHostParam](#tenantalterdbhostparam)
    - [Sample Project](#sample-project)

## How to use

### Initialize a project

1. Install the package by

```sh
go install github.com/retail-ai-inc/bean/v2/cmd/bean@latest
```

2. Create a project directory

```sh
mkdir myproject && cd myproject
```

3. Initialize the project using bean by

```sh
bean init myproject
```

or

```sh
bean init github.com/me/myproject
```

The above command will produce a nice directory structure with all necessary configuration files and code to start your project quickly. Now, let's build your project and start:

```sh
make build
./myproject start
```

<https://user-images.githubusercontent.com/61860255/155534942-b9ee6b70-ccf3-4cd6-a7c3-bc8bd5089626.mp4>

## Service-Repository Pattern

Bean is using service repository pattern for any database, file or external transaction. The `repository` provides a collection of interfaces to access data stored in a database, file system or external service. Data is returned in the form of `structs` or `interface`. The main idea to use `Repository Pattern` is to create a bridge between *models* and *handlers*. Here is a simple pictorial map to understand the service-repository pattern in a simple manner:

![Service_Repository_Pattern](static/service_repository_pattern.png)

## How To Create Repository File(s)

```sh
bean create repo login
bean create repo logout
```

Above two commands will create 2 repository files under `repositories` folder as `login.go` and `logout.go`.

Now let's associate the above repository files with a service called `auth`:

```sh
bean create service auth --repo login --repo logout
```

Above command will create a pre-defined sample service file under `services` folder as `auth.go` and automatically set the `type authService struct` and `func NewAuthService`.

Now you can run `make build` or `make build-slim` to compile your newly created service with repositories.

## One Liner To Create Service And Repositories

```sh
bean create service auth --repo login --repo profile,logout

OR

bean create service auth -r login -r profile,logout
```

Above command will create both service repository files if it doesn't exist and automatically set the association.

## How To Create Handler

```sh
bean create handler auth
```

Above command will create a pre-defined sample handler file under `handlers` folder as `auth.go`. Furthermore, if you already create an `auth` service with same name as `auth` then bean will automatically associate your handler with the auth service in `route.go`.

## Two Build Commands

Bean supporting 2 build commands:

- `make build` - This is usual go build command.
- `make build-slim` - This will create a slim down version of your binary by turning off the DWARF debugging information and Go symbol table. Furthemore, this will exclude file system paths from the resulting binary using `-trimpath`.

# Additional Features

## Built-In Logging

Bean has a pre-builtin logging system. If you open the `env.json` file from your project directory then you should see some configuration like below:

```json
"accessLog": {
  "on": true,
  "bodyDump": true,
  "path": "",
  "bodyDumpMaskParam": []
}
```

- `on` - Turn on/off the logging system. Default is `true`.
- `bodyDump` - Log the request-response body in the log file. This is helpful for debugging purpose. Default `true`
- `path` - Set the log file path. You can set like `logs/console.log`. Empty log path allow bean to log into `stdout`
- `bodyDumpMaskParam` - For security purpose if you don't wanna `bodyDump` some sensetive request parameter then you can add those as a string into the slice like `["password", "secret"]`. Default is empty.

The logger in bean is an instance of log.Logger interface from the `github.com/labstack/gommon/log` package [compatible with the standard log.Logger interface], there are multiple levels of logging such as `Debug`, `Info`, `Warn`, `Error` and to customize the formatting of the log messages. The logger also supports like `Debugf`, `Infof`, `Warnf`, `Errorf`, `Debugj`, `Infoj`, `Warnj`, `Errorj`.
The logger can be used in any of the layers `handler`, `service`, `repository`.

Example:-

  ```sh
  log.Logger().Debugf("This is a debug message for request %s", c.Request().URL.Path)
  ```

## Built-In testing

Bean provides a built-in testing framework to test your project. To run the test, you need to run the following command from your project directory like below:

```sh
bean test ./... -output=html -output-file=./your/proejct/report
```

If you want to know more about the testing command then you can run `bean test --help` from your project directory.

With helper functions under`test` package, you can easily test your project. The `test` package provides the following helper functions:-

- `SetupConfig` - This function will setup the config for your test. You can pass the `env.json` file path as a parameter. It will read config under `test` tag from the `env.json` file and set the config `TestConfig` for your test so that you can use the config in your test easily.

- `SetSeverity` - This function will set different severity levels for different tests so that you can make a result report of your testing more organized; you can see aggregated results of your tests based on the severity level either in JSON or HTML format.

- other util functions - These contains some helper functions like `SkipTestIfInSkipList`(that is supposed to be used along with `TestConfig`) to make your testing easier.

## Out of the Box Commands

A project built with bean also provides the following executable commands alongside the `start` command :-

1. gen secret
2. aes:encrypt/aes:decrypt
3. route list

### Generating Secret Key using gen secret command

In `env.json` file bean is maintaining a key called `secret`. This is a 32 character long random alphanumeric string. It's a multi purpose hash key or salt which you can use in your project to generate JWT, one way hash password, encrypt some private data or session. By default, `bean` is providing a secret however, you can generate a new one by entering the following command from your terminal:

```sh
./myproject gen secret
```

### Cryptography using the aes command

This command has two subcommands encrypt and decrypt used for encrypting and decrypting files using the AES encryption algorithm.
AES encryption is a symmetric encryption technique and it requires the use of a password to crypt the data, bean uses the password `secret` in `env.json` as the default password which is created when initializing the project. If you want to use a different password you can use `gen secret` command to update the key.

For encrypting data

```sh
./myproject aes:encrypt <string_to_encypt>
```

For decrypting data

```sh
./myproject aes:decrypt <string_to_decypt>
```

### Listing routes using the route list command

This command enables us to list the routes that the web server is currently serving alongside the correspoding methods and handler functions supporting them.

```sh
./myproject route list
```

## Make your own Commands

After initializing your project using `bean` you should able to see a directory like `commands/gopher/`. Inside this directory there is a file called `gopher.go`. This file represents the command as below:

```sh
./myproject gopher
```

Usually you don't need to modify `gopher.go` file. Now, let's create a new command file as `commands/gopher/helloworld.go` and paste the following codes:

```go
package gopher

import (
 "errors"
 "fmt"

 "github.com/retail-ai-inc/bean/v2"
 "github.com/retail-ai-inc/bean/v2/trace"
 "github.com/spf13/cobra"
)

func init() {
 cmd := &cobra.Command{
  Use:   "helloworld",
  Short: "Hello world!",
  Long:  `This command will just print hello world otherwise hello mars`,
  RunE: func(cmd *cobra.Command, args []string) error {
   NewHellowWorld()
   err := helloWorld("hello")
   if err != nil {
    // If you turn on `sentry` via env.json then the error will be captured by sentry otherwise ignore.
    trace.SentryCaptureException(cmd.Context(), err)
   }

   return err
  },
 }

 GopherCmd.AddCommand(cmd)
}

func NewHellowWorld() {
 // IMPORTANT: If you pass `false` then database connection will not be initialized.
 _ = initBean(false)
}

func helloWorld(h string) error {
 if h == "hello" {
  fmt.Println("hellow world")
  return nil
 }

 return errors.New("hello mars")
}
```

Now, compile your project and run the command as `./myproject gopher helloworld`. The command will just print the `hellow world`.

## Local K/V Memorystore

`Bean` supports a memory-efficient local K/V store. To configure it, you need to activate it from your `database` parameter in env.json like below:

```json
"memory": {
    "on": true,
    "delKeyAPI": {
        "endPoint": "/memory/key/:key",
        "authBearerToken": "<set_any_token_string_of_your_choice>"
    }
}
```

How to use in the code:

```go
import "github.com/retail-ai-inc/bean/v2/store/memory"

// Initialize the local memory store with key type `string` and value type `any`
m := memory.NewMemory()

// The third parameter is the `ttl`. O means forever.
m.SetMemory("Hello", "World", 0)

data, found := m.GetMemory("Hello")
if !found {
  // Do something
}

m.DelMemory("Hello")
```

The `delKeyAPI` parameter will help you proactively delete your local cache if you cache something from your database like SQL or NOSQL. For example, suppose you cache some access token in your local memory, which resides in your database, to avoid too many connections with your database. In that case, if your access token gets changed from the database, you can trigger the `delKeyAPI` endpoint with the key and `Bearer <authBearerToken>` as the header parameter then `bean` will delete the key from the local cache. Here, you must be careful if you run the `bean` application in a `k8s` container because then you have to trigger the `delKeyAPI` for all your pods separately by IP address from `k8s`.

## Useful Helper Functions

Please refer to the [`helpers` package](helpers/) in this codebase or [go doc](https://pkg.go.dev/github.com/retail-ai-inc/bean/v2/helpers) for more information.

## Bean Config

Bean provides the [`config.Config`](https://pkg.go.dev/github.com/retail-ai-inc/bean/v2/config#Config) struct to enable the user to tweak the configuration of their consumer project as per their requirement .
Bean configs default values are picked from the `env.json` file, but can be updated during runtime as well.
 <https://pkg.go.dev/github.com/retail-ai-inc/bean#Config>

<details>
  <summary>config.Config</summary>

  Some of the configurable parameters are:

- `Environment`: represents the environment in which the project is running (e.g. development, production, etc.)

- `DebugLogPath`: represents the path of the debug log file.

- `Secret`: represents a secret string key used for encryption and decryption in the project.
 Example Usecase:- while encoding/decoding JWTs.

- `HTTP`: represents a custom wrapper to deal with HTTP/HTTPS requests.
 The wrapper provides by default some common features but also some exclusive features like:-
  - `BodyLimit`: Sets the maximum allowed size for a request body, return `413 - Request Entity Too Large` if the size exceeds the limit.

  - `IsHttpsRedirect`: A boolean that represents whether to redirect HTTP requests to HTTPS or not.

  - `KeepAlive`: A boolean that represents whether to keep the HTTP connection alive or not.

  - `AllowedMethod`: A slice of strings that represents the allowed HTTP methods.
    Example:- `["DELETE","GET","POST","PUT"]`

  - `SSL`: used when web server uses HTTPS for communication.
   The SSL struct contains the following parameters:-
    - `On`: A boolean that represents whether SSL is enabled or not.

    - `CertFile`: represents the path of the certificate file.

    - `PrivFile`: represents the path of the private key file.

    - `MinTLSVersion`: represents the minimum TLS version required.

- `Prometheus`: represents the configuration for the Prometheus metrics.
 The Prometheus struct contains the following parameters:-
  - `On`: A boolean that represents whether Prometheus is enabled or not.

  - `SkipEndpoints`: represents the endpoints/paths to skip from Prometheus metrics.

  - `Subsystem`: represents the subsystem name for the Prometheus metrics. The default value is `echo` if empty.

</details>

## TenantAlterDbHostParam

The `TenantAlterDbHostParam` is helful in multitenant scenarios when we need to run some
cloudfunction or cron and you cannot connect your memorystore/SQL/mongo server from
cloudfunction/VM using the usual `host` ip.

  ```sh
  bean.TenantAlterDbHostParam = "gcpHost"
  ```

### Sample Project

A CRUD project that you can refer to understand how bean works with service repository pattern.
<https://github.com/RohitChaurasia97/movie_tracker>
