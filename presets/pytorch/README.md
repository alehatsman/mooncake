# PyTorch Preset

**Status:** ✓ Installed successfully

## Quick Start

```python
import torch

# Check version
print(torch.__version__)

# Check CUDA availability
print(f"CUDA available: {torch.cuda.is_available()}")
print(f"CUDA version: {torch.version.cuda}")
print(f"GPU count: {torch.cuda.device_count()}")

# Create tensor
x = torch.tensor([1, 2, 3])
print(x)
```

## Configuration

- **Installation:** Via pip or conda
- **CUDA support:** Auto-detected if available
- **Device:** CPU by default, GPU if CUDA available

## Basic Operations

```python
import torch

# Create tensors
x = torch.zeros(3, 4)
y = torch.ones(3, 4)
z = torch.rand(3, 4)

# Tensor operations
result = x + y
result = torch.matmul(x, y.T)

# Move to GPU
if torch.cuda.is_available():
    x = x.cuda()
    # or
    x = x.to('cuda')

# Move back to CPU
x = x.cpu()

# Convert to numpy
np_array = x.numpy()

# From numpy
x = torch.from_numpy(np_array)
```

## Neural Network Example

```python
import torch
import torch.nn as nn
import torch.optim as optim

# Define model
class Net(nn.Module):
    def __init__(self):
        super(Net, self).__init__()
        self.fc1 = nn.Linear(784, 128)
        self.fc2 = nn.Linear(128, 10)

    def forward(self, x):
        x = torch.relu(self.fc1(x))
        x = self.fc2(x)
        return x

# Create model
model = Net()

# Move to GPU
if torch.cuda.is_available():
    model = model.cuda()

# Loss and optimizer
criterion = nn.CrossEntropyLoss()
optimizer = optim.Adam(model.parameters(), lr=0.001)

# Training loop
for epoch in range(10):
    optimizer.zero_grad()
    outputs = model(inputs)
    loss = criterion(outputs, labels)
    loss.backward()
    optimizer.step()
```

## Data Loading

```python
from torch.utils.data import Dataset, DataLoader

# Custom dataset
class MyDataset(Dataset):
    def __init__(self, data, labels):
        self.data = data
        self.labels = labels

    def __len__(self):
        return len(self.data)

    def __getitem__(self, idx):
        return self.data[idx], self.labels[idx]

# Create dataloader
dataset = MyDataset(data, labels)
dataloader = DataLoader(dataset, batch_size=32, shuffle=True)

# Iterate
for batch_data, batch_labels in dataloader:
    # Training code
    pass
```

## Save/Load Model

```python
# Save model
torch.save(model.state_dict(), 'model.pth')

# Load model
model = Net()
model.load_state_dict(torch.load('model.pth'))
model.eval()
```

## Common Modules

```python
# Layers
nn.Linear(in_features, out_features)
nn.Conv2d(in_channels, out_channels, kernel_size)
nn.BatchNorm2d(num_features)
nn.Dropout(p=0.5)

# Activations
nn.ReLU()
nn.Sigmoid()
nn.Softmax(dim=1)

# Loss functions
nn.CrossEntropyLoss()
nn.MSELoss()
nn.BCELoss()

# Optimizers
optim.SGD(model.parameters(), lr=0.01)
optim.Adam(model.parameters(), lr=0.001)
optim.AdamW(model.parameters(), lr=0.001)
```

## TorchVision (if installed)

```python
import torchvision
import torchvision.transforms as transforms

# Transforms
transform = transforms.Compose([
    transforms.Resize(256),
    transforms.CenterCrop(224),
    transforms.ToTensor(),
    transforms.Normalize(mean=[0.485, 0.456, 0.406],
                       std=[0.229, 0.224, 0.225])
])

# Datasets
trainset = torchvision.datasets.CIFAR10(
    root='./data', train=True,
    download=True, transform=transform)

# Pretrained models
model = torchvision.models.resnet50(pretrained=True)
```

## Debugging

```python
# Check gradients
for name, param in model.named_parameters():
    if param.grad is not None:
        print(f"{name}: {param.grad.norm()}")

# Memory usage
print(torch.cuda.memory_allocated())
print(torch.cuda.memory_reserved())

# Clear cache
torch.cuda.empty_cache()
```

## Uninstall

```yaml
- preset: pytorch
  with:
    state: absent
```

**Note:** Model checkpoints preserved after uninstall.
