# metal-install

Render installation artifacts (boot.ipxe, kickstart / preseed /
user-data, post-install scripts) for physical-server installation.

Three subcommands:

- `metal-install server` -- HTTP API that renders per-node artifacts
  on demand and serves them over HTTP so an installer can fetch them
- `metal-install render` -- generate per-node artifacts from one
  InstallSpec on the command line, without running the server
- `metal-install snapshot` -- generate artifacts for every machine x
  OS combination declared in `compatibility.yml`, into a directory
  tree suitable for committing to a repository and reviewing with
  `git diff`

All three subcommands share the same `internal/render` package; the
output for a given (DataSet, Spec) input is identical regardless of
which subcommand is invoked.

## metal-install render

Generate artifacts for a single InstallSpec.

```
./metal-install render \
  -spec examples/spec/almalinux.yml \
  -data-dir examples/data \
  -output-dir ./out
```

`examples/spec/` contains one spec file per OS (`almalinux.yml`,
`debian.yml`, `ubuntu.yml`), useful for trying a single combination
quickly.

Writes `out/<node_id>/` containing:

- `boot.ipxe`
- the installer configuration files for the chosen OS (`kickstart.ks`
  for RHEL family, `preseed.cfg` for Debian, `user-data` + `meta-data`
  for Ubuntu autoinstall)
- `post/<script>.sh` -- one file per script declared under `setup.post`
  in the OS YAML

## metal-install snapshot

Generate artifacts for every (machine, OS) combination declared in
`compatibility.yml`, using a fixed sample spec (RFC 5737 IPs,
placeholder password and SSH key, a sample MAC as node_id). The point
of snapshot output is to show *template rendering* differences across
combinations; the artifacts are not meant to be deployed.

```
./metal-install snapshot \
  -data-dir examples/data \
  -output-dir ./examples/snapshot
```

Output layout:

```
examples/snapshot/
  <machine_id>/
    <os_id>/
      boot.ipxe
      <installer config files>
      post/<scripts>.sh
```

The intended workflow:

1. Edit a template, an OS YAML, or a machine YAML.
2. Run `metal-install snapshot -data-dir examples/data
   -output-dir examples/snapshot`.
3. `git diff examples/snapshot/` to inspect the impact of the change
   on every supported combination.
4. Commit the data change and the snapshot update together.

## metal-install server

Serve artifacts over HTTP.

```
./metal-install server \
  -data-dir examples/data \
  -state-dir ./state \
  -listen :8080
```

Endpoints:

- `POST /nodes` accept InstallSpec (JSON), render, record under
  `state/`
- `GET /nodes` list active node IDs
- `GET /nodes/{node_id}` show one node's spec
- `DELETE /nodes/{node_id}` remove a node from the registry
- `GET /configs/{node_id}/{file...}` serve a generated artifact
  (the file path may include subdirectory components, e.g.
  `post/ssh.sh`)
- `GET /health` readiness

POST example:

```
curl -X POST -H 'Content-Type: application/json' \
  -d '{"machine":"qemu_vm","os":"almalinux100","node_id":"...","ipv4_addr":"...",...}' \
  http://localhost:8080/nodes
```

The data directory is loaded once at startup; restart the server to
pick up changes (matches the deploy model where releases are deployed
as a new directory and symlink-switched).

The state directory holds two things:

```
state/nodes/<node_id>.json      one file per active node
state/configs/<node_id>/        generated artifacts per node
```

On DELETE the node JSON is removed; the configs/ directory is kept
for audit and debugging. Operators can `rm -rf` old configs later.

## URL convention

metal-install serves per-node artifacts under:

```
/configs/<node_id>/boot.ipxe
/configs/<node_id>/<installer config files>
/configs/<node_id>/post/<script>.sh
```

The installer config files depend on the OS: `kickstart.ks` for RHEL
family, `preseed.cfg` for Debian, `user-data` and `meta-data` for
Ubuntu autoinstall. Which files an OS produces is declared in its
OS YAML file under `configs:` and `setup.post:`.

The choice of `node_id` is a caller convention; common choices include
the MAC address in colon-less hex (e.g. `${mac:hexhyp}` in iPXE
produces `52-54-00-aa-bb-cc`). metal-install does not interpret the
`node_id` value.

## Referencing metal-install from iPXE

A typical bootstrap chain pulls a per-node boot script:

```
#!ipxe
chain http://<install-server>/configs/${mac:hexhyp}/boot.ipxe
```

See `examples/ipxe/bootstrap.ipxe` for a complete example. How a node
arrives at `/configs/<node_id>/boot.ipxe` is outside the scope of
metal-install.

## Data directory layout

```
data/
  compatibility.yml         allowed (machine, os) combinations
  env.yml                   install-server URLs and defaults
  machines/<id>.yml         hardware-specific data (NICs, bonds,
                              partitioning, etc.)
  os/<id>.yml               OS-specific data (kernel/initrd, packages,
                              which templates to render)
  templates/                template tree, loaded recursively;
                              subdirectory names have no semantic
                              meaning to the loader and are
                              conventionally grouped by OS family
    rhel/                     boot.ipxe.tmpl, kickstart.ks.tmpl
    debian/                   boot.ipxe.tmpl, preseed.cfg.tmpl
    ubuntu/                   boot.ipxe.tmpl, user-data.tmpl,
                                meta-data.tmpl
    setup/post/               post-install scripts named by each
                                OS's setup.post list
```

## Design principles

- The same render code is used by the CLI, the snapshot generator,
  and the HTTP server; identical input produces identical output
- The data directory is the source of truth; restarting the server
  is the way to pick up changes
- HA is not a requirement; single-process, file-based state
- metal-install does not track install completion or lifecycle. It
  renders and serves artifacts for whatever nodes are registered.
  Registering and deregistering a node is the caller's concern
- Post-install logic lives in small, single-responsibility shell
  scripts under `setup.post`; the installer config (kickstart /
  preseed / user-data) names which scripts to fetch and run, in
  what order. Each script is independently editable and testable.

## License

This project is licensed under the [MIT License](./LICENSE).
