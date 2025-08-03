Selectel for [`libdns`](https://github.com/libdns/libdns)
=======================

[![Go Reference](https://pkg.go.dev/badge/test.svg)](https://pkg.go.dev/github.com/jfk9w-go/libdns-selectel)

This package implements the [libdns interfaces](https://github.com/libdns/libdns) for Selectel, allowing you to manage DNS records.

## Usage

[Selectel API documentation](https://docs.selectel.ru/en/api/authorization/#iam-token-project-scoped) describes the process of obtaining
`X-Auth-Token` necessary for managing DNS zones & records. Note that the library handles
authentication by itself, you only to provide service user authentication data.

An example of usage can be seen in `integration_test.go`. 
To run clone the `.env.template` to a file named `.env` and populate with the required data.
