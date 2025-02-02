# k8ssh

k8ssh is a tool that allows you to SSH into Kubernetes pods using service accounts. It provides a secure way to access your Kubernetes pods without exposing them directly.

## Features

- SSH into Kubernetes pods
- SFTP support
- Impersonation using Kubernetes service accounts
- Command execution with annotations

## Installation

To install k8ssh, you need to have Go installed on your machine. Then, you can clone the repository and build the binary.

```sh
git clone https://github.com/guilhem/k8ssh.git
cd k8ssh
go build -o k8ssh ./cmd
```

## Usage

To start the k8ssh server, use the `serve` command:

```sh
./k8ssh serve --address :2222
```

This will start the server on port 2222. You can then SSH into your Kubernetes pods using the following command:

```sh
ssh -i /path/to/private/key user@pod.namespace@localhost -p 2222
```

### SFTP

To use SFTP, you can use the following command:

```sh
sftp -i /path/to/private/key user@pod.namespace@localhost -P 2222
```

## User Management

k8ssh uses the service account name as the username when you SSH into a pod. You can configure the public key for the service account using the `ssh.barpilot.io/publickey` annotation.

To add user with a public key, you can use the following configuration:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-user-ssh
  annotations:
    ssh.barpilot.io/publickey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQD..."
```

## Access Control

k8ssh uses Kubernetes RBAC to control access to the pods. You can grant access to user service account to a specific pod by adding the following RBAC configuration:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-access
rules:
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
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
    ssh.barpilot.io/prefix-command: "echo Prefix;"
```

## Impersonation

k8ssh use impersonation to enforce the Kubernetes RBAC rules. When you SSH into a pod, k8ssh will impersonate the service account associated with the pod. This allows you to access the pod with the same permissions as the service account.

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
```

You can then bind this role to a user or group using the following RBAC configuration to your k8ssh service account:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8sssh-impersonate
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
