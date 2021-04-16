# k8ssh

Connect to kubernetes pod with SSH.
You can see it as a SSH <-> `kubectl exec` gateway.

## Usage

If we have a pod like that:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: k8sssh-example
spec:
  containers:
    - name: ubuntu
      image: ubuntu
      command:
        - sleep
        - infinity
```

We can create a Connection:

```yaml
apiVersion: k8ssh.barpilot.io/v1alpha1
kind: Connection
metadata:
  name: myuser
spec:
  pod:
    name: k8sssh-example
    namespace: default
  command: ["bash"]
  password: "example"
```

A user named `myuser` with a password `example` will be connected to pod `k8sssh-example` in namespace `default` and, without any command specified on user side it will spawn a `bash` shell.
