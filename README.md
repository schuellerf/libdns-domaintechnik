# Domaintechnik for [libdns](https://github.com/libdns/libdns)

Go module: [`github.com/schuellerf/libdns-domaintechnik`](https://github.com/schuellerf/libdns-domaintechnik)

This package implements the [libdns](https://github.com/libdns/libdns) interfaces for [Domaintechnik](https://www.domaintechnik.at/) ACME DNS-01.

It only supports publishing and removing `_acme-challenge` TXT records through:

- `POST …/acme.present`
- `POST …/acme.cleanup`

It cannot list or manage other record types. `GetRecords` / `SetRecords` are not implemented.

## Configuration

```go
import "github.com/schuellerf/libdns-domaintechnik"

provider := &domaintechnik.Provider{
    APIToken: "dt1_your_key_here",
    // APIEndpoint defaults to https://manage.domaintechnik.at/dns-api/v1
}
```

Check a token without changing DNS:

```go
err := provider.Ping(ctx)
```

## ACME DNS-01

Caddy (and other libdns clients) pass a relative name `_acme-challenge` and a zone such as `schueller.pro.`. This package sends `name=_acme-challenge.schueller.pro` and `value=<TXT>` as `application/x-www-form-urlencoded`, with `Authorization: Bearer <token>`.

For Caddy, use [`github.com/schuellerf/caddy-dns-domaintechnik`](https://github.com/schuellerf/caddy-dns-domaintechnik).
