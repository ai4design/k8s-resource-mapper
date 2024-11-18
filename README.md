# Kubernetes Resource Mapper

A Go-based tool for visualizing and mapping relationships between Kubernetes resources across namespaces. This tool provides a clear, ASCII-based visualization of your cluster's resources and their interconnections.

## 🙏 Special Thanks

Special thanks to **Leandro "Big Dog" Silva** for enabling this project with guidance and expertise. Your contributions to making Kubernetes resource visualization more accessible are greatly appreciated! 🐕

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
git clone https://github.com/yourusername/k8s-resource-mapper.git
cd k8s-resource-mapper
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
export KUBE