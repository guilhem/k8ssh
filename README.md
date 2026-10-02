# k8ssh

k8ssh is a tool that allows you to SSH into Kubernetes pods using service accounts. It provides a secure way to access your Kubernetes pods without exposing them directly.

## Features

- SSH into Kubernetes pods
- SFTP support
- Impersonation using Kubernetes service accounts
- Command execution with annotations

## Installation

Build with Go 1.26 or newer:

```sh
git clone https://github.com/guilhem/k8ssh.git
cd k8ssh
go build -o k8ssh .
```

## Usage

The server uses your current kubeconfig (or its in-cluster service account).
Create a persistent, unencrypted host key and start the server:

```sh
ssh-keygen -t ed25519 -N '' -f ./host_key
./k8ssh serve --address :2222 --hostkey ./host_key
```

Keep `host_key` private and reuse it across restarts. Without `--hostkey`, k8ssh
generates an ephemeral RSA key and clients will see a different host identity after
a restart. OpenSSH, PKCS#1, EC and PKCS#8 private key files are supported; encrypted
keys are not.

Connect using the login name `<service-account>@<pod>.<namespace>`:

```sh
ssh -p 2222 -i /path/to/private/key -l 'my-user-ssh@my-pod.default' localhost
```

### SFTP

To use SFTP, you can use the following command:

```sh
sftp -P 2222 -i /path/to/private/key -o User='my-user-ssh@my-pod.default' localhost
```

The target container must provide `/usr/lib/sftp-server` for SFTP.

## User Management

k8ssh uses the service account name as the username when you SSH into a pod. You can configure the public key for the service account using the `ssh.barpilot.io/publickey` annotation.

The service account must be in the target pod namespace and have a valid SSH
public key annotation. Missing or malformed keys deny authentication. To add a
user, replace the example key with the contents of that user's `.pub` file:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-user-ssh
  annotations:
    ssh.barpilot.io/publickey: "ssh-ed25519 AAAA... user@example"
```

## Access Control

The authenticated service account needs access to `pods/exec`. Both `get`
(WebSocket) and `create` (SPDY fallback) are used. This Role limits access to a
single pod; create it and the RoleBinding in that pod's namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-access
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["get", "create"]
  resourceNames:
  - my-pod
```

You can then bind this role to a user or group using the following RBAC configuration:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: pod-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: pod-access
subjects:
- kind: ServiceAccount
  name: my-user-ssh
  namespace: default
```

## Command Management

You can configure the command to execute when the user logs in using the `ssh.barpilot.io/command` annotation.

You can also configure a prefix command to execute before the main command using the `ssh.barpilot.io/prefix-command` annotation.

### Pod Example

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  annotations:
    ssh.barpilot.io/command: "bash"
    ssh.barpilot.io/prefix-command: "env TERM=xterm-256color"
```

## Impersonation

k8ssh impersonates the **authenticated login service account**, which may differ
from the pod's own service account. Kubernetes authorizes each exec request using
that identity. The server's own identity needs `get` access to pods and service
accounts for lookup, plus permission to impersonate the allowed login accounts.
Only trusted administrators should be able to change their public key annotations.

To impersonate a service account, you need to have the `impersonate` permission in the Kubernetes RBAC rules. You can grant this permission by adding the following rule to your RBAC configuration:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: impersonate
rules:
- apiGroups: [""]
  resources: ["serviceaccounts"]
  verbs: ["impersonate"]
  resourceNames: ["my-user-ssh"]
- apiGroups: [""]
  resources: ["pods", "serviceaccounts"]
  verbs: ["get"]
```

You can then bind this role to a user or group using the following RBAC configuration to your k8ssh service account:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8ssh-impersonate
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: impersonate
subjects:
- kind: ServiceAccount
  name: k8ssh
  namespace: default
```

## Configuration

k8ssh uses annotations on service accounts and pods to configure the SSH and SFTP commands. The following annotations are supported:

- `ssh.barpilot.io/publickey`: The public key for the service account.
- `ssh.barpilot.io/command`: The command to execute when the user logs in.
- `ssh.barpilot.io/prefix-command`: A prefix command to execute before the main command.

Commands are split into arguments, not interpreted by a shell. A prefix such as
`env TERM=xterm-256color` is prepended to both SSH and SFTP commands. Pod
annotations take precedence over service account annotations.

## Development

```sh
go test -race ./...
go vet ./...
go build -o k8ssh .
```

CI runs these checks on Go 1.26 and 1.27. Unit tests do not establish connectivity
or authorization against a live Kubernetes cluster.

## Contributing

We welcome contributions to k8ssh. To contribute, please follow these steps:

1. Fork the repository
2. Create a new branch (`git checkout -b feature-branch`)
3. Make your changes
4. Commit your changes (`git commit -am 'Add new feature'`)
5. Push to the branch (`git push origin feature-branch`)
6. Create a new Pull Request

## License

k8ssh is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for more information.
