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
