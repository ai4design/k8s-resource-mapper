# Kubernetes Resource Mapper

A Go-based tool for visualizing and mapping relationships between Kubernetes resources across namespaces. This tool provides a clear, ASCII-based visualization of your cluster's resources and their interconnections.

## 🙏 Special Thanks

Special thanks to **Leandro "Big Dog" Silva** (@leandrosilva) for enabling this project with guidance and expertise. Your contributions to making Kubernetes resource visualization more accessible are greatly appreciated! 🐕

## 📋 Features

- 🔍 Comprehensive resource discovery and mapping
- 🔗 Service-to-pod relationship visualization
- 📊 ConfigMap usage tracking
- 🌐 Ingress routing visualization
- 🎨 Color-coded output for better readability
- 🚀 Namespace filtering options
- 📡 Real-time cluster state analysis

## 🌟 Resources Tracked

- Deployments
- HorizontalPodAutoscalers (HPA)
- Services
- Ingresses
- Pods
- ConfigMaps
- Namespace relationships

## 📦 Prerequisites

- Go 1.19 or later
- Access to a Kubernetes cluster
- `kubectl` configured with cluster access
- Valid kubeconfig file

## 🛠️ Installation

1. Clone the repository:
```bash
git clone https://github.com/ai4design/k8s-resource-mapper.git
cd src
```

2. Install dependencies:
```bash
go mod tidy
```

3. Build the binary:
```bash
go build -o k8s-resource-mapper
```

## 🚀 Usage

Basic usage:
```bash
# Show all namespaces
./k8s-resource-mapper

# Show specific namespace
./k8s-resource-mapper -n default

# Exclude specific namespaces
./k8s-resource-mapper --exclude-ns kube-system --exclude-ns kube-public

# Show help
./k8s-resource-mapper -h
```

### Command Line Options

| Flag | Alternative | Description |
|------|-------------|-------------|
| `-n` | `--namespace` | Process only the specified namespace |
| `--exclude-ns` | - | Exclude specified namespaces |
| `-h` | `--help` | Show help message |

## 📝 Sample Output

```plaintext
External Traffic
│
▼
[Ingress Layer]
├── api-ingress
│   ----> Service: auth-service
│   ----> Service: product-service
│
▼
[Service Layer]
├── auth-service
│   ----> Pod: auth-service-xxx-yyy
├── product-service
│   ----> Pod: product-service-xxx-yyy
```

## 💻 Development

### Building from Source

```bash
# Get dependencies
go mod download

# Build
go build -o k8s-resource-mapper

# Run tests (if available)
go test ./...
```

### Local Development Setup

1. Install Go 1.19 or later
2. Configure kubectl with your cluster
3. Set KUBECONFIG environment variable if needed:
```bash
export KUBECONFIG=~/.kube/config
```

## 🐳 Docker Support

Build the container:
```bash
docker build -t k8s-resource-mapper .
```

Run with your kubeconfig:
```bash
docker run -v ~/.kube/config:/root/.kube/config k8s-resource-mapper
```

## 🔧 Configuration

The tool uses your current kubeconfig context. You can specify a different kubeconfig file using the KUBECONFIG environment variable:

```bash
export KUBECONFIG=/path/to/your/kubeconfig
```

## 🐛 Troubleshooting

Common issues and solutions:

1. **Kubeconfig not found**
   ```bash
   export KUBECONFIG=~/.kube/config
   ```

2. **Permission Issues**
   ```bash
   # Verify cluster access
   kubectl auth can-i get pods --all-namespaces
   ```

3. **Build Errors**
   ```bash
   go mod tidy
   go mod verify
   ```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch
   ```bash
   git checkout -b feature/amazing-feature
   ```
3. Commit your changes
   ```bash
   git commit -m 'Add amazing feature'
   ```
4. Push to the branch
   ```bash
   git push origin feature/amazing-feature
   ```
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🌟 Acknowledgments

- Leandro "Big Dog" Silva for the inspiration and guidance
- Kubernetes client-go library documentation
- The Go community for excellent tooling

## 📞 Support

- Create an issue for bug reports
- Start a discussion for feature requests
- Check existing issues for known problems

---
Made with ❤️ by the Kubernetes community
