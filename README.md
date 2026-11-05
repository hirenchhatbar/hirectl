# hirectl

Dockerised, Makefile-driven Go project skeleton. Only Docker and `make` needed on the host.

## Commands

```
make setup   # build dev image
make build   # bin/<app>
make run
make test
make image   # production image <app>:latest
make bump    # PART=patch|minor|major, or V=x.y.z
make help    # all targets
```
