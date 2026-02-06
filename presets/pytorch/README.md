# PyTorch - Deep Learning Framework

Open-source machine learning framework with GPU acceleration, dynamic computational graphs, and production deployment capabilities.

## Quick Start

```yaml
- preset: pytorch
```

## Features

- **Dynamic computation graphs**: Define-by-run execution model for flexibility
- **GPU acceleration**: Native CUDA support for NVIDIA GPUs, MPS for Apple Silicon
- **TorchScript**: JIT compilation for production deployment
- **Distributed training**: Multi-GPU and multi-node training support
- **Rich ecosystem**: TorchVision (vision), TorchAudio (audio), TorchText (NLP)
- **ONNX export**: Model interoperability with other frameworks
- **Production ready**: TorchServe for model serving
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage

```python
import torch

# Check version and device
print(torch.__version__)
print(f"CUDA available: {torch.cuda.is_available()}")
print(f"CUDA version: {torch.version.cuda}")

# Create tensors
x = torch.tensor([1, 2, 3])
y = torch.zeros(3, 4)
z = torch.rand(3, 4)

# Move to GPU
if torch.cuda.is_available():
    x = x.cuda()  # or x.to('cuda')

# Basic operations
result = x + y
matrix_mult = torch.matmul(x, y.T)
```

## Advanced Configuration

```yaml
# Install PyTorch with specific configuration
- preset: pytorch
  with:
    install_method: conda           # or pip
    gpu_support: auto               # auto, cpu, cuda, mps
    install_torchvision: true       # Image processing
    install_torchaudio: false       # Audio processing
  register: pytorch_result

# Verify installation
- name: Test PyTorch
  shell: |
    python -c "import torch; print(torch.__version__); print(torch.cuda.is_available())"
  register: test_result

# Install with CUDA support
- preset: pytorch
  with:
    gpu_support: cuda
    install_torchvision: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (`present` or `absent`) |
| install_method | string | conda | Installation method (`pip` or `conda`) |
| gpu_support | string | auto | GPU support (`auto`, `cpu`, `cuda`, `mps`) |
| install_torchvision | bool | true | Install torchvision for image processing |
| install_torchaudio | bool | false | Install torchaudio for audio processing |

## Platform Support

- ✅ Linux (apt, dnf for dependencies, pip/conda for PyTorch)
- ✅ macOS (Homebrew for dependencies, pip/conda for PyTorch)
- ✅ Windows (conda/pip, requires Visual Studio)
- ✅ CUDA (NVIDIA GPUs)
- ✅ MPS (Apple Silicon M1/M2/M3)

## Configuration

- **Installation directory**: `$CONDA_PREFIX/lib/python3.x/site-packages/torch/` (conda) or virtualenv site-packages (pip)
- **Cache directory**: `~/.cache/torch/` (model checkpoints, datasets)
- **CUDA libraries**: System CUDA installation or bundled with PyTorch
- **Environment variables**:
  - `TORCH_HOME`: Cache directory override
  - `CUDA_VISIBLE_DEVICES`: GPU selection

## Real-World Examples

### Computer Vision Model Training

```yaml
# Setup PyTorch with GPU support
- preset: pytorch
  with:
    gpu_support: cuda
    install_torchvision: true
    install_method: conda
  become: true

# Train image classifier
- name: Train ResNet model
  shell: |
    python train.py \
      --model resnet50 \
      --dataset imagenet \
      --epochs 100 \
      --batch-size 256 \
      --lr 0.1
  environment:
    CUDA_VISIBLE_DEVICES: "0,1,2,3"
```

Training script example:

```python
import torch
import torch.nn as nn
import torch.optim as optim
import torchvision
import torchvision.transforms as transforms
from torch.utils.data import DataLoader

# Setup device
device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

# Data preparation
transform = transforms.Compose([
    transforms.RandomResizedCrop(224),
    transforms.RandomHorizontalFlip(),
    transforms.ToTensor(),
    transforms.Normalize(mean=[0.485, 0.456, 0.406],
                       std=[0.229, 0.224, 0.225])
])

trainset = torchvision.datasets.ImageFolder(
    root='./data/train',
    transform=transform
)
trainloader = DataLoader(trainset, batch_size=32, shuffle=True, num_workers=4)

# Model
model = torchvision.models.resnet50(pretrained=False, num_classes=10)
model = model.to(device)

# Training
criterion = nn.CrossEntropyLoss()
optimizer = optim.SGD(model.parameters(), lr=0.01, momentum=0.9)

for epoch in range(10):
    running_loss = 0.0
    for i, (inputs, labels) in enumerate(trainloader):
        inputs, labels = inputs.to(device), labels.to(device)

        optimizer.zero_grad()
        outputs = model(inputs)
        loss = criterion(outputs, labels)
        loss.backward()
        optimizer.step()

        running_loss += loss.item()

    print(f"Epoch {epoch+1}, Loss: {running_loss/len(trainloader):.4f}")

# Save model
torch.save(model.state_dict(), 'resnet50.pth')
```

### Natural Language Processing

```yaml
# Install PyTorch for NLP workloads
- preset: pytorch
  with:
    install_method: pip
    gpu_support: auto
```

Transformer model example:

```python
import torch
import torch.nn as nn

class TransformerClassifier(nn.Module):
    def __init__(self, vocab_size, d_model=512, nhead=8, num_layers=6):
        super().__init__()
        self.embedding = nn.Embedding(vocab_size, d_model)
        self.pos_encoder = PositionalEncoding(d_model)

        encoder_layer = nn.TransformerEncoderLayer(
            d_model=d_model,
            nhead=nhead,
            dim_feedforward=2048
        )
        self.transformer = nn.TransformerEncoder(encoder_layer, num_layers)
        self.fc = nn.Linear(d_model, num_classes)

    def forward(self, x):
        x = self.embedding(x) * np.sqrt(self.d_model)
        x = self.pos_encoder(x)
        x = self.transformer(x)
        x = x.mean(dim=1)  # Global average pooling
        return self.fc(x)

# Training
model = TransformerClassifier(vocab_size=10000, num_classes=2)
model = model.to(device)

optimizer = optim.Adam(model.parameters(), lr=0.001)
criterion = nn.CrossEntropyLoss()

# Train loop...
```

### Distributed Training

```yaml
# Setup PyTorch on multiple nodes
- preset: pytorch
  hosts: gpu-cluster
  with:
    gpu_support: cuda
    install_method: conda
  become: true

# Launch distributed training
- name: Run DDP training
  shell: |
    torchrun \
      --nproc_per_node=4 \
      --nnodes=2 \
      --node_rank={{ node_rank }} \
      --master_addr={{ master_addr }} \
      --master_port=29500 \
      train_ddp.py
  environment:
    CUDA_VISIBLE_DEVICES: "0,1,2,3"
```

DDP training script:

```python
import torch
import torch.distributed as dist
from torch.nn.parallel import DistributedDataParallel as DDP
from torch.utils.data.distributed import DistributedSampler

def setup(rank, world_size):
    dist.init_process_group("nccl", rank=rank, world_size=world_size)

def cleanup():
    dist.destroy_process_group()

def train(rank, world_size):
    setup(rank, world_size)

    # Model
    model = YourModel().to(rank)
    ddp_model = DDP(model, device_ids=[rank])

    # Data
    sampler = DistributedSampler(
        dataset,
        num_replicas=world_size,
        rank=rank
    )
    dataloader = DataLoader(
        dataset,
        batch_size=32,
        sampler=sampler
    )

    # Training loop
    for epoch in range(num_epochs):
        sampler.set_epoch(epoch)
        for batch in dataloader:
            # Training step...
            pass

    cleanup()

if __name__ == "__main__":
    world_size = torch.cuda.device_count()
    torch.multiprocessing.spawn(
        train,
        args=(world_size,),
        nprocs=world_size
    )
```

### Model Deployment with TorchServe

```yaml
# Install PyTorch for production serving
- preset: pytorch
  with:
    install_method: conda

# Install TorchServe
- name: Install TorchServe
  shell: |
    pip install torchserve torch-model-archiver torch-workflow-archiver

# Archive model
- name: Create model archive
  shell: |
    torch-model-archiver \
      --model-name resnet50 \
      --version 1.0 \
      --model-file model.py \
      --serialized-file resnet50.pth \
      --handler image_classifier \
      --extra-files index_to_name.json \
      --export-path model-store/

# Start TorchServe
- name: Run TorchServe
  shell: |
    torchserve --start \
      --model-store model-store \
      --models resnet50=resnet50.mar
```

Inference request:

```bash
# Predict
curl http://localhost:8080/predictions/resnet50 \
  -T image.jpg

# Health check
curl http://localhost:8080/ping
```

## Common Operations

```python
# Tensor operations
x = torch.randn(3, 4)
y = torch.randn(4, 5)
z = torch.matmul(x, y)

# Automatic differentiation
x = torch.tensor([2.0], requires_grad=True)
y = x ** 2 + 3 * x + 1
y.backward()
print(x.grad)  # dy/dx

# Save/load model
torch.save(model.state_dict(), 'model.pth')
model.load_state_dict(torch.load('model.pth'))

# Save entire model
torch.save(model, 'complete_model.pth')
model = torch.load('complete_model.pth')

# Export to ONNX
torch.onnx.export(
    model,
    dummy_input,
    "model.onnx",
    input_names=['input'],
    output_names=['output']
)
```

## Mixed Precision Training

```python
from torch.cuda.amp import autocast, GradScaler

# Setup
scaler = GradScaler()

# Training loop
for epoch in range(epochs):
    for inputs, labels in dataloader:
        optimizer.zero_grad()

        # Forward pass with autocast
        with autocast():
            outputs = model(inputs)
            loss = criterion(outputs, labels)

        # Backward pass with gradient scaling
        scaler.scale(loss).backward()
        scaler.step(optimizer)
        scaler.update()
```

## TorchVision Usage

```python
import torchvision.models as models
import torchvision.transforms as transforms

# Pretrained models
resnet50 = models.resnet50(pretrained=True)
vgg16 = models.vgg16(pretrained=True)
efficientnet = models.efficientnet_b0(pretrained=True)

# Transforms
transform = transforms.Compose([
    transforms.Resize(256),
    transforms.CenterCrop(224),
    transforms.ToTensor(),
    transforms.Normalize(
        mean=[0.485, 0.456, 0.406],
        std=[0.229, 0.224, 0.225]
    )
])

# Datasets
from torchvision.datasets import CIFAR10, ImageNet

trainset = CIFAR10(
    root='./data',
    train=True,
    download=True,
    transform=transform
)
```

## Agent Use

- Train deep learning models for computer vision tasks
- Build and deploy NLP models (transformers, LSTMs)
- Implement reinforcement learning agents
- Fine-tune pretrained models for transfer learning
- Distributed training across multiple GPUs/nodes
- Model serving and production deployment
- Research and experimentation with custom architectures
- AutoML and neural architecture search

## Troubleshooting

### CUDA out of memory

Reduce memory usage:

```python
# Reduce batch size
batch_size = 16  # Instead of 32

# Use gradient accumulation
accumulation_steps = 4
for i, (inputs, labels) in enumerate(dataloader):
    outputs = model(inputs)
    loss = criterion(outputs, labels) / accumulation_steps
    loss.backward()

    if (i + 1) % accumulation_steps == 0:
        optimizer.step()
        optimizer.zero_grad()

# Clear cache
torch.cuda.empty_cache()

# Use mixed precision
from torch.cuda.amp import autocast
with autocast():
    outputs = model(inputs)
```

### Model not learning

Check common issues:

```python
# Verify data loading
for inputs, labels in dataloader:
    print(inputs.shape, labels.shape)
    break

# Check gradients
for name, param in model.named_parameters():
    if param.grad is not None:
        print(f"{name}: {param.grad.norm()}")

# Verify loss function
print(f"Loss: {loss.item()}")

# Check learning rate
print(f"LR: {optimizer.param_groups[0]['lr']}")
```

### Slow training

Optimize performance:

```python
# Use num_workers for data loading
dataloader = DataLoader(dataset, batch_size=32, num_workers=4)

# Pin memory for GPU
dataloader = DataLoader(dataset, pin_memory=True)

# Use channels_last memory format
model = model.to(memory_format=torch.channels_last)

# Enable cudnn benchmarking
torch.backends.cudnn.benchmark = True
```

### Installation issues

```bash
# Verify CUDA installation
nvidia-smi
nvcc --version

# Check PyTorch CUDA version
python -c "import torch; print(torch.version.cuda)"

# Reinstall with specific CUDA version
conda install pytorch torchvision torchaudio pytorch-cuda=11.8 -c pytorch -c nvidia
```

## Uninstall

```yaml
- preset: pytorch
  with:
    state: absent
```

**Note**: Model checkpoints in `~/.cache/torch/` are preserved after uninstall.

## Resources

- Official docs: https://pytorch.org/docs/stable/
- Tutorials: https://pytorch.org/tutorials/
- GitHub: https://github.com/pytorch/pytorch
- Examples: https://github.com/pytorch/examples
- TorchServe: https://pytorch.org/serve/
- Search: "pytorch tutorial", "pytorch distributed training", "pytorch torchserve"
