# 🤝 Contributing

## 🚀 Getting Started

```
git clone https://github.com/trustbuilder/terraform-provider-trustbuilder.git
cd terraform-provider-trustbuilder
```

## 🛠️ Requirements
- [asdf](https://asdf-vm.com/guide/getting-started.html#_1-install-asdf)

OR:

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.23
- [golangci-lint](https://github.com/golangci/golangci-lint?tab=readme-ov-file#install-golangci-lint)
- [pre-commit](https://pre-commit.com/#installation)
- [make](https://www.gnu.org/software/make/manual/make.html)


## 🐧 Development setup for Linux
1. Install requirements with asdf (if you chose this solution)
    ```
    asdf current
    # Install a version of terraform globally for the acceptance tests
    asdf set -u terraform latest
    asdf install
    ```
2. If golang is installed with asdf:
    ```bash
    export GOBIN="$HOME/go/bin"
    ```
3. Use the provider development version
    You can plan and apply locally with the version in development of this provider with:
    ```bash
    make install
    export TF_CLI_CONFIG_FILE=$PWD/dev.tfrc
    ```
  It avoids to modify directly your `~/.terraformrc` file
4. pre-commit
    ```bash
    pre-commit install
    pre-commit run --all-files
    ```

## Building The Provider
1. Clone the repository: `git clone https://github.com/trustbuilder/terraform-provider-trustbuilder.git`
2. Enter the repository directory: `cd terraform-provider-trustbuilder`
3. Build the provider : `make build`


## 🐛 Debugging
* For [VSCode](https://registry.terraform.io/providers/DigitecGalaxus/dg-servicebus/latest/docs/guides/howto-debugprovider)
* When you run the tests, if you want to see the **all the logs**, you have to set `TF_LOG="DEBUG"`.


## ✅ Execute the tests
* unit tests:
  ```bash
  make test
  ```
* acceptance tests:
  ```bash
  make testacc
  ```


## 📦 Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.


## 📄 Generate the provider documentation

```bash
make generate
```
