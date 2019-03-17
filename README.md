# hansel

A remote code execution engine written in Go.  It uses the SSH protocol to secure connection between systems.
Only Linux supported at this time.

## Usage

### Server
This will sart the server on your host listening on port 4545.
In addition to starting the server it will also generate keys located in ~/.hansel

```bash
> hansel serve -p 4545 
```

### Client
This will connect the client to the server, it uses exponential backoff to prevent a thundering herd.
The initial connect to the server will fail until you copy the key from `~/.hansel/pending_keys` to `~/.hansel/authorized_keys`

Once you do that you should see a lot of scrolling data as actual command runners have not been implemented yet.

```bash
> hansel client -h localhost -p 4545
```

#### TODO:
Get remote execution running
Figure out some sort of templating engine(HCL&HIL?)
Implement a server connector to dynamically accept/regect keys and exert control
Implement key revocation