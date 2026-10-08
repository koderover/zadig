# DinD Proxy Configuration

The proxy API accepts these optional fields:

| Field | Default | Scope |
| --- | --- | --- |
| `enable_dind_proxy` | `false` | Enable proxying image pulls in the managed DinD daemon. |
| `dind_no_proxy` | `""` | Additional addresses that DinD must access directly. |

DinD proxying is independent of `enable_repo_proxy`. Existing configurations
without `enable_dind_proxy` do not enable managed image-pull proxies. Proxy type `no`
disables the managed DinD proxy even if `enable_dind_proxy` remains set.
Disabling or deleting a managed proxy removes its environment variables.

Example proxy settings:

```json
{
  "type": "https",
  "address": "proxy.example.com",
  "port": 8080,
  "enable_dind_proxy": true,
  "dind_no_proxy": "harbor.corp\nregistry.internal:5000, .corp"
}
```

`dind_no_proxy` accepts comma, space, tab or newline separators. Empty entries
are ignored, and entries are joined with commas when generating `NO_PROXY`.
It applies only to DinD, not to Git, Docker build arguments or install scripts.

The default bypass list contains `localhost`, `127.0.0.1`, `.svc`,
`.cluster.local`, `10.0.0.0/8`, `172.16.0.0/12` and `192.168.0.0/16`.
All registries registered in Zadig retain direct access, including Docker Hub.

CIDR entries match IP addresses in the request URL. They do not resolve a
hostname and compare its DNS result against the CIDR. An unregistered internal
registry such as `harbor.corp` must therefore be listed in `dind_no_proxy`, even
if it resolves to a private IP address.

The pre-merge field `no_proxy` has been replaced by `dind_no_proxy`. Development
configurations using that field must be resubmitted with the new field name;
there is no released-data migration or alternate field interpretation.

Proxy-triggered DinD synchronization runs in the background and logs failures.
Registry saves retain their synchronous local DinD update path. Each cluster is
synchronized under its own lock, and the latest registry and proxy settings are
read inside that lock, so concurrent saves cannot restore an older snapshot.
Updates can roll DinD pods when their proxy environment changes.
